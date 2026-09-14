package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"technik-server/config"
	"technik-server/database"
	"technik-server/dto"
	"technik-server/prisma/db"

	"github.com/gin-gonic/gin"
)

type TechnikHandler struct {
	cfg *config.Config
}

func NewTechnikHandler(cfg *config.Config) *TechnikHandler {
	return &TechnikHandler{cfg: cfg}
}

// NominateTechnikPride handles single or batch student nominations for Technik Pride Award
func (h *TechnikHandler) NominateTechnikPride(c *gin.Context) {
	ctx := context.Background()
	currentYear := time.Now().Year()

	// Try reading as batch nomination request
	var batchReq dto.TechnikPrideBatchNominationRequest
	if err := c.ShouldBindJSON(&batchReq); err == nil && len(batchReq.Nominations) > 0 {
		year := batchReq.AcademicYear
		if year <= 0 {
			year = currentYear
		}

		var createdResponses []dto.TechnikPrideNominationResponse
		for _, nom := range batchReq.Nominations {
			schoolID := nom.SchoolID
			if schoolID == "" {
				schoolID = batchReq.SchoolID
			}

			params := []db.TechnikPrideNominationSetParam{
				db.TechnikPrideNomination.ClassCategory.Set(nom.ClassCategory),
				db.TechnikPrideNomination.SupportingDocument.Set(nom.SupportingDocument),
			}
			if schoolID != "" {
				params = append(params, db.TechnikPrideNomination.School.Link(
					db.SchoolDetails.ID.Equals(schoolID),
				))
			}

			res, err := database.Client.TechnikPrideNomination.CreateOne(
				db.TechnikPrideNomination.StudentName.Set(nom.StudentName),
				db.TechnikPrideNomination.Class.Set(nom.Class),
				db.TechnikPrideNomination.Gender.Set(nom.Gender),
				db.TechnikPrideNomination.AchievementCategory.Set(nom.AchievementCategory),
				db.TechnikPrideNomination.AchievementTitle.Set(nom.AchievementTitle),
				db.TechnikPrideNomination.BriefDescription.Set(nom.BriefDescription),
				params...,
			).Exec(ctx)

			if err == nil && res != nil {
				schID, _ := res.SchoolID()
				clsCat, _ := res.ClassCategory()
				supDoc, _ := res.SupportingDocument()
				createdResponses = append(createdResponses, dto.TechnikPrideNominationResponse{
					ID:                  res.ID,
					SchoolID:            schID,
					AcademicYear:        res.AcademicYear,
					StudentName:         res.StudentName,
					Class:               res.Class,
					ClassCategory:       clsCat,
					Gender:              res.Gender,
					AchievementCategory: res.AchievementCategory,
					AchievementTitle:    res.AchievementTitle,
					BriefDescription:    res.BriefDescription,
					SupportingDocument:  supDoc,
					CreatedAt:           res.CreatedAt,
					UpdatedAt:           res.UpdatedAt,
				})
			}
		}

		c.JSON(http.StatusCreated, dto.TechnikPrideAuthResponse{
			Success:     true,
			Message:     "Technik Pride Award nominations submitted successfully",
			Total:       len(createdResponses),
			Nominations: createdResponses,
		})
		return
	}

	// Single nomination fallback
	var singleReq dto.TechnikPrideNominationRequest
	if err := c.ShouldBindJSON(&singleReq); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	year := singleReq.AcademicYear
	if year <= 0 {
		year = currentYear
	}

	params := []db.TechnikPrideNominationSetParam{
		db.TechnikPrideNomination.AcademicYear.Set(year),
		db.TechnikPrideNomination.ClassCategory.Set(singleReq.ClassCategory),
		db.TechnikPrideNomination.SupportingDocument.Set(singleReq.SupportingDocument),
	}

	if singleReq.SchoolID != "" {
		params = append(params, db.TechnikPrideNomination.School.Link(
			db.SchoolDetails.ID.Equals(singleReq.SchoolID),
		))
	}

	res, err := database.Client.TechnikPrideNomination.CreateOne(
		db.TechnikPrideNomination.StudentName.Set(singleReq.StudentName),
		db.TechnikPrideNomination.Class.Set(singleReq.Class),
		db.TechnikPrideNomination.Gender.Set(singleReq.Gender),
		db.TechnikPrideNomination.AchievementCategory.Set(singleReq.AchievementCategory),
		db.TechnikPrideNomination.AchievementTitle.Set(singleReq.AchievementTitle),
		db.TechnikPrideNomination.BriefDescription.Set(singleReq.BriefDescription),
		params...,
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create nomination: " + err.Error()})
		return
	}

	schID, _ := res.SchoolID()
	clsCat, _ := res.ClassCategory()
	supDoc, _ := res.SupportingDocument()

	c.JSON(http.StatusCreated, dto.TechnikPrideAuthResponse{
		Success: true,
		Message: "Technik Pride Award nomination created successfully",
		Total:   1,
		Nominations: []dto.TechnikPrideNominationResponse{
			{
				ID:                  res.ID,
				SchoolID:            schID,
				AcademicYear:        res.AcademicYear,
				StudentName:         res.StudentName,
				Class:               res.Class,
				ClassCategory:       clsCat,
				Gender:              res.Gender,
				AchievementCategory: res.AchievementCategory,
				AchievementTitle:    res.AchievementTitle,
				BriefDescription:    res.BriefDescription,
				SupportingDocument:  supDoc,
				CreatedAt:           res.CreatedAt,
				UpdatedAt:           res.UpdatedAt,
			},
		},
	})
}

// GetTechnikPrideNominations fetches nominations for current year or filtered query
func (h *TechnikHandler) GetTechnikPrideNominations(c *gin.Context) {
	ctx := context.Background()
	yearStr := c.Query("year")
	schoolID := c.Query("schoolId")

	year := time.Now().Year()
	if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
		year = y
	}

	var list []db.TechnikPrideNominationModel
	var err error

	if schoolID != "" {
		list, err = database.Client.TechnikPrideNomination.FindMany(
			db.TechnikPrideNomination.AcademicYear.Equals(year),
			db.TechnikPrideNomination.SchoolID.Equals(schoolID),
		).Exec(ctx)
	} else {
		list, err = database.Client.TechnikPrideNomination.FindMany(
			db.TechnikPrideNomination.AcademicYear.Equals(year),
		).Exec(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch nominations: " + err.Error()})
		return
	}

	var responses []dto.TechnikPrideNominationResponse
	for _, res := range list {
		schID, _ := res.SchoolID()
		clsCat, _ := res.ClassCategory()
		supDoc, _ := res.SupportingDocument()
		responses = append(responses, dto.TechnikPrideNominationResponse{
			ID:                  res.ID,
			SchoolID:            schID,
			AcademicYear:        res.AcademicYear,
			StudentName:         res.StudentName,
			Class:               res.Class,
			ClassCategory:       clsCat,
			Gender:              res.Gender,
			AchievementCategory: res.AchievementCategory,
			AchievementTitle:    res.AchievementTitle,
			BriefDescription:    res.BriefDescription,
			SupportingDocument:  supDoc,
			CreatedAt:           res.CreatedAt,
			UpdatedAt:           res.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, dto.TechnikPrideAuthResponse{
		Success:     true,
		Message:     "Nominations retrieved successfully",
		Total:       len(responses),
		Nominations: responses,
	})
}

// RegisterOlympiad handles student registration in array for Olympiad tracks
func (h *TechnikHandler) RegisterOlympiad(c *gin.Context) {
	var req dto.OlympiadRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.APIError{Success: false, Error: FormatValidationError(err)})
		return
	}

	ctx := context.Background()
	currentYear := time.Now().Year()
	year := req.AcademicYear
	if year <= 0 {
		year = currentYear
	}

	// Create Olympiad Registration Batch Header
	var regParams []db.OlympiadRegistrationSetParam
	regParams = append(regParams, db.OlympiadRegistration.AcademicYear.Set(year))
	regParams = append(regParams, db.OlympiadRegistration.TotalStudents.Set(len(req.Students)))

	if req.SchoolID != "" {
		regParams = append(regParams, db.OlympiadRegistration.School.Link(
			db.SchoolDetails.ID.Equals(req.SchoolID),
		))
	}

	registration, err := database.Client.OlympiadRegistration.CreateOne(
		regParams...,
	).Exec(ctx)

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to create Olympiad registration: " + err.Error()})
		return
	}

	// Create student entries in array
	var studentResponses []dto.OlympiadStudentResponse
	for _, st := range req.Students {
		stParams := []db.OlympiadStudentSetParam{
			db.OlympiadStudent.ClassCategory.Set(st.ClassCategory),
		}

		stRes, err := database.Client.OlympiadStudent.CreateOne(
			db.OlympiadStudent.OlympiadRegistration.Link(
				db.OlympiadRegistration.ID.Equals(registration.ID),
			),
			db.OlympiadStudent.StudentName.Set(st.StudentName),
			db.OlympiadStudent.Class.Set(st.Class),
			db.OlympiadStudent.Gender.Set(st.Gender),
			db.OlympiadStudent.OlympiadTrack.Set(st.OlympiadTrack),
			stParams...,
		).Exec(ctx)

		if err == nil && stRes != nil {
			cat, _ := stRes.ClassCategory()
			studentResponses = append(studentResponses, dto.OlympiadStudentResponse{
				ID:            stRes.ID,
				StudentName:   stRes.StudentName,
				Class:         stRes.Class,
				ClassCategory: cat,
				Gender:        stRes.Gender,
				OlympiadTrack: stRes.OlympiadTrack,
				CreatedAt:     stRes.CreatedAt,
			})
		}
	}

	schID, _ := registration.SchoolID()
	c.JSON(http.StatusCreated, dto.OlympiadBatchResponse{
		Success: true,
		Message: "Olympiad student registrations submitted successfully",
		Registration: dto.OlympiadRegistrationResponse{
			ID:            registration.ID,
			SchoolID:      schID,
			AcademicYear:  registration.AcademicYear,
			TotalStudents: len(studentResponses),
			Students:      studentResponses,
			CreatedAt:     registration.CreatedAt,
		},
	})
}

// GetOlympiadRegistrations fetches registered olympiad students/batches
func (h *TechnikHandler) GetOlympiadRegistrations(c *gin.Context) {
	ctx := context.Background()
	schoolID := c.Query("schoolId")
	yearStr := c.Query("year")

	year := time.Now().Year()
	if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
		year = y
	}

	var registrations []db.OlympiadRegistrationModel
	var err error

	if schoolID != "" {
		registrations, err = database.Client.OlympiadRegistration.FindMany(
			db.OlympiadRegistration.AcademicYear.Equals(year),
			db.OlympiadRegistration.SchoolID.Equals(schoolID),
		).With(
			db.OlympiadRegistration.Students.Fetch(),
		).Exec(ctx)
	} else {
		registrations, err = database.Client.OlympiadRegistration.FindMany(
			db.OlympiadRegistration.AcademicYear.Equals(year),
		).With(
			db.OlympiadRegistration.Students.Fetch(),
		).Exec(ctx)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.APIError{Success: false, Error: "Failed to fetch registrations: " + err.Error()})
		return
	}

	var results []dto.OlympiadRegistrationResponse
	for _, reg := range registrations {
		schID, _ := reg.SchoolID()
		var stResps []dto.OlympiadStudentResponse
		for _, st := range reg.Students() {
			cat, _ := st.ClassCategory()
			stResps = append(stResps, dto.OlympiadStudentResponse{
				ID:            st.ID,
				StudentName:   st.StudentName,
				Class:         st.Class,
				ClassCategory: cat,
				Gender:        st.Gender,
				OlympiadTrack: st.OlympiadTrack,
				CreatedAt:     st.CreatedAt,
			})
		}

		results = append(results, dto.OlympiadRegistrationResponse{
			ID:            reg.ID,
			SchoolID:      schID,
			AcademicYear:  reg.AcademicYear,
			TotalStudents: len(stResps),
			Students:      stResps,
			CreatedAt:     reg.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Olympiad registrations retrieved successfully",
		"total":         len(results),
		"registrations": results,
	})
}
