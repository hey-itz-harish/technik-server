package handlers

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalQuestionPaperParser(t *testing.T) {
	sampleDoc := `
TECHNIK ACADEMIC ASSESSMENT 2026
Subject: Science & Robotics
Class: Grade 5
Total Marks: 50

SECTION I: Choose the Best Answer (5 x 1 = 5 Marks)
1. Which sensor detects obstacles using sound? [1 Mark]
2. What supplies power to the microcontroller? [1 Mark]
3. Which actuator converts electrical energy into motion? [1 Mark]
4. What programming loop repeats instructions until stopped? [1 Mark]
5. Which communication protocol uses two wires: SDA and SCL? [1 Mark]

SECTION II: Short Answer Reasoning (5 x 2 = 10 Marks)
6. Explain the difference between digital and analog input pins. (2 Marks)
7. Describe how an ultrasonic sensor calculates obstacle distance. (2 Marks)
8. Why is motor driver circuit required between Arduino and DC motor? (2 Marks)
9. Define open-loop control versus closed-loop control system. (2 Marks)
10. What is pulse width modulation (PWM) used for in robotics? (2 Marks)

SECTION III: Analytical Application (3 x 5 = 15 Marks)
11. Design a simple obstacle avoiding wheeled mobile robot algorithm. (5 Marks)
12. Compare ultrasonic sensors and infrared sensors in varying light conditions. (5 Marks)
13. Formulate the formula to determine battery operational life for a 2000mAh pack. (5 Marks)
`

	parsed := ParseQuestionPaperLocally(sampleDoc)
	if parsed == nil {
		t.Fatalf("Expected parsed data, got nil")
	}

	if len(parsed.Sessions) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(parsed.Sessions))
	}

	if len(parsed.RomanNumerals) != 3 {
		t.Errorf("Expected 3 roman numerals, got %d", len(parsed.RomanNumerals))
	} else {
		if parsed.RomanNumerals[0] != "I" || parsed.RomanNumerals[1] != "II" || parsed.RomanNumerals[2] != "III" {
			t.Errorf("Unexpected roman numerals: %v", parsed.RomanNumerals)
		}
	}

	if parsed.NumberOfQuestions != 13 {
		t.Errorf("Expected 13 questions, got %d", parsed.NumberOfQuestions)
	}

	if parsed.TotalMark != 30 {
		t.Errorf("Expected 30 marks allocated, got %.1f", parsed.TotalMark)
	}
}

func TestDocxExtraction(t *testing.T) {
	// Create a mock .docx file in temporary directory
	tempDir := t.TempDir()
	docxPath := filepath.Join(tempDir, "sample_exam.docx")

	f, err := os.Create(docxPath)
	if err != nil {
		t.Fatalf("Failed to create mock file: %v", err)
	}

	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}

	mockXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>SECTION I: Multiple Choice Questions (5 Marks)</w:t></w:r></w:p>
    <w:p><w:r><w:t>1. What is an actuator? (1 Mark)</w:t></w:r></w:p>
    <w:p><w:r><w:t>2. What is a sensor? (1 Mark)</w:t></w:r></w:p>
  </w:body>
</w:document>`

	if _, err := w.Write([]byte(mockXML)); err != nil {
		t.Fatalf("Failed to write mock XML: %v", err)
	}
	_ = zw.Close()
	_ = f.Close()

	extracted, err := ExtractTextFromDocument(docxPath)
	if err != nil {
		t.Fatalf("Failed to extract text from docx: %v", err)
	}

	if len(extracted) == 0 {
		t.Fatalf("Expected extracted text, got empty")
	}

	parsed := ParseQuestionPaperLocally(extracted)
	if len(parsed.Sessions) == 0 || parsed.NumberOfQuestions != 2 {
		t.Errorf("Expected 2 questions parsed, got %d", parsed.NumberOfQuestions)
	}
}

