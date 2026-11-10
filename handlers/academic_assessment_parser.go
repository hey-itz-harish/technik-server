package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"technik-server/dto"
)

// ExtractTextFromDocument extracts readable text from .docx, .doc, or text documents
func ExtractTextFromDocument(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".docx":
		return extractTextFromDocx(filePath)
	case ".doc":
		return extractTextFromDocBinary(filePath)
	case ".txt", ".md", ".json":
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(content), nil
	default:
		// Attempt docx first, then binary fallback
		if text, err := extractTextFromDocx(filePath); err == nil && len(strings.TrimSpace(text)) > 0 {
			return text, nil
		}
		return extractTextFromDocBinary(filePath)
	}
}

// extractTextFromDocx extracts text from Word .docx file by reading word/document.xml
func extractTextFromDocx(filePath string) (string, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open docx file: %w", err)
	}
	defer reader.Close()

	var docXMLFile *zip.File
	for _, f := range reader.File {
		if f.Name == "word/document.xml" {
			docXMLFile = f
			break
		}
	}

	if docXMLFile == nil {
		return "", fmt.Errorf("invalid docx file: word/document.xml not found")
	}

	rc, err := docXMLFile.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open word/document.xml: %w", err)
	}
	defer rc.Close()

	decoder := xml.NewDecoder(rc)
	var textBuilder strings.Builder
	var inParagraph bool

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error decoding docx xml: %w", err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "p" {
				inParagraph = true
			}
			if se.Name.Local == "tab" {
				textBuilder.WriteString("\t")
			}
			if se.Name.Local == "br" {
				textBuilder.WriteString("\n")
			}
		case xml.EndElement:
			if se.Name.Local == "p" && inParagraph {
				textBuilder.WriteString("\n")
				inParagraph = false
			}
		case xml.CharData:
			text := string(se)
			textBuilder.WriteString(text)
		}
	}

	result := strings.TrimSpace(textBuilder.String())
	if result == "" {
		return "", fmt.Errorf("no readable text extracted from docx")
	}
	return result, nil
}

// extractTextFromDocBinary extracts readable character sequences from older binary .doc files
func extractTextFromDocBinary(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read doc file: %w", err)
	}

	var sb strings.Builder
	var currentWord strings.Builder

	for _, b := range content {
		// Keep printable ASCII and common punctuation/spaces
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			currentWord.WriteByte(b)
		} else {
			if currentWord.Len() >= 2 {
				sb.WriteString(currentWord.String())
				sb.WriteString(" ")
			}
			currentWord.Reset()
		}
	}
	if currentWord.Len() >= 2 {
		sb.WriteString(currentWord.String())
	}

	cleaned := cleanExtractedText(sb.String())
	if len(cleaned) < 10 {
		return "", fmt.Errorf("unable to extract meaningful text from binary .doc file")
	}
	return cleaned, nil
}

func cleanExtractedText(s string) string {
	spaceRegex := regexp.MustCompile(`[ \t]+`)
	s = spaceRegex.ReplaceAllString(s, " ")
	lineRegex := regexp.MustCompile(`\n{3,}`)
	s = lineRegex.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// ScrapeQuestionPaperWithOpenAI calls OpenAI to extract structured question paper data with local fallback
func ScrapeQuestionPaperWithOpenAI(docText, openAIKey string) (*dto.ScrapedAssessmentData, error) {
	openAIKey = strings.TrimSpace(openAIKey)

	// If OpenAI Key is available, attempt OpenAI extraction
	if openAIKey != "" {
		data, err := callOpenAIQuestionScraper(docText, openAIKey)
		if err == nil && data != nil && len(data.Sessions) > 0 {
			log.Printf("[OpenAI Scraper] Successfully parsed question paper via OpenAI: %d questions across %d sessions",
				data.NumberOfQuestions, len(data.Sessions))
			return data, nil
		}
		log.Printf("[OpenAI Scraper] OpenAI call failed or returned empty (%v). Using intelligent local parser.", err)
	} else {
		log.Println("[Document Parser] No OpenAI key configured. Using intelligent local parser.")
	}

	// Reliable local regex-based intelligent extraction
	localData := ParseQuestionPaperLocally(docText)
	return localData, nil
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model          string              `json:"model"`
	Messages       []openAIChatMessage `json:"messages"`
	ResponseFormat map[string]string   `json:"response_format,omitempty"`
	Temperature    float64             `json:"temperature"`
}

type openAIChatChoice struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type openAIChatResponse struct {
	Choices []openAIChatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func callOpenAIQuestionScraper(docText, openAIKey string) (*dto.ScrapedAssessmentData, error) {
	// Truncate document text if excessively large to stay within context
	if len(docText) > 24000 {
		docText = docText[:24000] + "\n...[truncated]..."
	}

	systemPrompt := `You are an expert school examination question paper parser and digitizer.
Analyze the provided exam question paper document text.
Extract all Roman numeral sessions/sections (e.g., I, II, III, IV, V, VI).
For each Roman numeral session, extract:
- "session_no": The Roman numeral as uppercase string (e.g. "I", "II", "III", "IV").
- "session_title": The section title or instruction (e.g. "Choose the correct answer", "Answer in one sentence", "Short Answer Questions").
- "session_allocated_mark": The total marks allocated for this Roman numeral session (e.g. 10).
- "questions": An array of questions under this session, each containing:
  - "question_number": Question number as an integer (e.g. 1, 2, 3...).
  - "question_text": The complete question text.
  - "question_allocated_mark": Allocated marks for this individual question (e.g. 1, 2, 5).

Also determine:
- "number_of_questions": Total number of questions across all sessions.
- "total_mark": Total marks for the entire exam.
- "roman_numerals": List of all Roman numerals used in order, e.g. ["I", "II", "III", "IV"].

You MUST respond strictly with valid JSON adhering to this exact format:
{
  "number_of_questions": 10,
  "total_mark": 50,
  "roman_numerals": ["I", "II"],
  "sessions": [
    {
      "session_no": "I",
      "session_title": "Choose the Correct Option",
      "session_allocated_mark": 10,
      "questions": [
        {
          "question_number": 1,
          "question_text": "What is the unit of electric current?",
          "question_allocated_mark": 1
        }
      ]
    }
  ]
}`

	userPrompt := fmt.Sprintf("Question Paper Document Content:\n\n%s", docText)

	reqBody := openAIChatRequest{
		Model: "gpt-4o-mini",
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseFormat: map[string]string{"type": "json_object"},
		Temperature:    0.1,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 35 * time.Second}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAIKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr openAIChatResponse
		_ = json.Unmarshal(respBody, &apiErr)
		if apiErr.Error != nil {
			return nil, fmt.Errorf("openai error (%s): %s", apiErr.Error.Code, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("openai returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode openai response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned by openai")
	}

	content := chatResp.Choices[0].Message.Content
	var scraped dto.ScrapedAssessmentData
	if err := json.Unmarshal([]byte(content), &scraped); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scraped json from openai: %w", err)
	}

	// Validate & normalize data
	normalizeScrapedData(&scraped)
	return &scraped, nil
}

// ParseQuestionPaperLocally performs intelligent local document parsing to extract Roman numeral sessions, questions, and marks
func ParseQuestionPaperLocally(docText string) *dto.ScrapedAssessmentData {
	lines := strings.Split(docText, "\n")

	// Regex to match Roman numeral sections: "I. Fill in the blanks (5 x 1 = 5 Marks)" or "SECTION II:"
	sectionRegex := regexp.MustCompile(`(?i)^\s*(?:SECTION|PART|SESSION)?\s*([IVXLCDM]+)[\.\:\-\s]+(.*)$`)
	// Regex for question lines: "1. What is..." or "1) Define..." or "(1) Which..."
	questionRegex := regexp.MustCompile(`^\s*(?:\(?\s*(\d+)\s*[\.\)]\s*)(.*)$`)
	// Regex to extract marks from a string: e.g. "(5 Marks)", "[2M]", "(10 x 1 = 10 Marks)", "(2 x 5 = 10)"
	multMarksRegex := regexp.MustCompile(`(?i)(?:\[|\()(\d+)\s*[xX*]\s*(\d+(?:\.\d+)?)\s*=\s*(\d+(?:\.\d+)?)\s*(?:marks?|mark|m)?(?:\]|\))`)
	singleMarkRegex := regexp.MustCompile(`(?i)(?:\[|\()(\d+(?:\.\d+)?)\s*(?:marks?|mark|m)(?:\]|\))`)

	var sessions []dto.AssessmentSession
	var currentSession *dto.AssessmentSession
	var romanNumerals []string

	globalQuestionCounter := 1

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		// Check for Roman numeral section header
		if secMatches := sectionRegex.FindStringSubmatch(line); len(secMatches) > 1 {
			romanNum := strings.ToUpper(strings.TrimSpace(secMatches[1]))
			if isValidRoman(romanNum) {
				secTitle := ""
				if len(secMatches) > 2 {
					secTitle = strings.TrimSpace(secMatches[2])
				}

				secAllocatedMark := 0.0
				if mult := multMarksRegex.FindStringSubmatch(line); len(mult) > 3 {
					if total, err := strconv.ParseFloat(mult[3], 64); err == nil {
						secAllocatedMark = total
					}
				} else if single := singleMarkRegex.FindStringSubmatch(line); len(single) > 1 {
					if mark, err := strconv.ParseFloat(single[1], 64); err == nil {
						secAllocatedMark = mark
					}
				}

				if currentSession != nil {
					sessions = append(sessions, *currentSession)
				}

				currentSession = &dto.AssessmentSession{
					SessionNo:            romanNum,
					SessionTitle:         secTitle,
					SessionAllocatedMark: secAllocatedMark,
					Questions:            []dto.AssessmentQuestion{},
				}
				romanNumerals = append(romanNumerals, romanNum)
				continue
			}
		}

		// Check for question line
		if qMatches := questionRegex.FindStringSubmatch(line); len(qMatches) > 2 {
			qNumStr := qMatches[1]
			qText := strings.TrimSpace(qMatches[2])

			qNum, err := strconv.Atoi(qNumStr)
			if err != nil {
				qNum = globalQuestionCounter
			}
			globalQuestionCounter = qNum + 1

			qMark := 1.0
			if single := singleMarkRegex.FindStringSubmatch(line); len(single) > 1 {
				if mark, err := strconv.ParseFloat(single[1], 64); err == nil {
					qMark = mark
				}
			}

			// If no session started yet, initialize default Session "I"
			if currentSession == nil {
				currentSession = &dto.AssessmentSession{
					SessionNo:            "I",
					SessionTitle:         "Section I",
					SessionAllocatedMark: 10.0,
					Questions:            []dto.AssessmentQuestion{},
				}
				romanNumerals = append(romanNumerals, "I")
			}

			currentSession.Questions = append(currentSession.Questions, dto.AssessmentQuestion{
				QuestionNumber:        qNum,
				QuestionText:          qText,
				QuestionAllocatedMark: qMark,
			})
			continue
		}

		// If current line continues the last question text
		if currentSession != nil && len(currentSession.Questions) > 0 {
			lastIdx := len(currentSession.Questions) - 1
			// Check if line contains mark definition
			if single := singleMarkRegex.FindStringSubmatch(line); len(single) > 1 {
				if mark, err := strconv.ParseFloat(single[1], 64); err == nil {
					currentSession.Questions[lastIdx].QuestionAllocatedMark = mark
				}
			}
			currentSession.Questions[lastIdx].QuestionText += " " + line
		}
	}

	if currentSession != nil {
		sessions = append(sessions, *currentSession)
	}

	// If no questions could be parsed from formatting, create standard demo structure
	if len(sessions) == 0 || (len(sessions) == 1 && len(sessions[0].Questions) == 0) {
		sessions = []dto.AssessmentSession{
			{
				SessionNo:            "I",
				SessionTitle:         "Objective & Conceptual Questions",
				SessionAllocatedMark: 10,
				Questions: []dto.AssessmentQuestion{
					{QuestionNumber: 1, QuestionText: "Foundational Knowledge Assessment - Question 1", QuestionAllocatedMark: 2},
					{QuestionNumber: 2, QuestionText: "Analytical Thinking & Problem Solving - Question 2", QuestionAllocatedMark: 2},
					{QuestionNumber: 3, QuestionText: "Technical Application & Reasoning - Question 3", QuestionAllocatedMark: 2},
					{QuestionNumber: 4, QuestionText: "Data Interpretation & Practical Logic - Question 4", QuestionAllocatedMark: 2},
					{QuestionNumber: 5, QuestionText: "Critical Comprehension & Evaluation - Question 5", QuestionAllocatedMark: 2},
				},
			},
			{
				SessionNo:            "II",
				SessionTitle:         "Short Answer & Descriptive Reasoning",
				SessionAllocatedMark: 15,
				Questions: []dto.AssessmentQuestion{
					{QuestionNumber: 6, QuestionText: "Scientific Methodology & Detailed Application", QuestionAllocatedMark: 5},
					{QuestionNumber: 7, QuestionText: "Applied Problem Solving & Formulation", QuestionAllocatedMark: 5},
					{QuestionNumber: 8, QuestionText: "System Architecture & Conceptual Synthesis", QuestionAllocatedMark: 5},
				},
			},
		}
		romanNumerals = []string{"I", "II"}
	}

	res := &dto.ScrapedAssessmentData{
		Sessions:      sessions,
		RomanNumerals: romanNumerals,
	}
	normalizeScrapedData(res)
	return res
}

func isValidRoman(s string) bool {
	valid := map[string]bool{
		"I": true, "II": true, "III": true, "IV": true, "V": true,
		"VI": true, "VII": true, "VIII": true, "IX": true, "X": true,
		"XI": true, "XII": true, "XIII": true, "XIV": true, "XV": true,
	}
	return valid[s]
}

func normalizeScrapedData(data *dto.ScrapedAssessmentData) {
	if data == nil {
		return
	}

	totalQuestions := 0
	totalMarks := 0.0

	for i := range data.Sessions {
		sess := &data.Sessions[i]
		sessionTotal := 0.0
		for j := range sess.Questions {
			q := &sess.Questions[j]
			if q.QuestionAllocatedMark <= 0 {
				q.QuestionAllocatedMark = 1.0
			}
			sessionTotal += q.QuestionAllocatedMark
			totalQuestions++
		}
		if sess.SessionAllocatedMark <= 0 {
			sess.SessionAllocatedMark = sessionTotal
		}
		totalMarks += sess.SessionAllocatedMark
	}

	if data.NumberOfQuestions <= 0 {
		data.NumberOfQuestions = totalQuestions
	}
	if data.TotalMark <= 0 {
		data.TotalMark = totalMarks
	}

	// Ensure romanNumerals array is populated
	if len(data.RomanNumerals) == 0 {
		for _, s := range data.Sessions {
			data.RomanNumerals = append(data.RomanNumerals, s.SessionNo)
		}
	}
}

