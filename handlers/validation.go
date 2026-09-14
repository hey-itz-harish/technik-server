package handlers

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

// friendlyFieldNames maps Go struct field names (as reported by the
// validator) to the plain-English label a user would recognize from the
// form they filled in.
var friendlyFieldNames = map[string]string{
	"SchoolName":             "School Name",
	"Board":                  "Board",
	"State":                  "State",
	"District":               "District",
	"City":                   "City",
	"Address":                "School Address",
	"Pincode":                "Pincode",
	"Email":                  "Email",
	"Phone":                  "Phone Number",
	"SchoolMobile":           "School Mobile Number",
	"Password":               "Password",
	"PrincipalName":          "Principal Name",
	"CoordinatorName":        "Coordinator Name",
	"CoordinatorDesignation": "Coordinator Designation",
	"CoordinatorMobile":      "Coordinator Mobile Number",
	"CoordinatorEmail":       "Coordinator Email",
	"OTP":                    "Verification Code",
	"Code":                   "Verification Code",
	"Token":                  "Activation Link",
	"ActToken":               "Setup Link",
	"Name":                   "Name",
	"Role":                   "Role",
	"StudentName":            "Student Name",
	"Grade":                  "Grade",
}

// FormatValidationError converts a Gin/validator binding error into a
// plain-English message suitable for showing directly to an end user,
// instead of the raw "Key: 'X.Y' Error:Field validation..." developer output.
func FormatValidationError(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		messages := make([]string, 0, len(ve))
		seen := make(map[string]bool)
		for _, fe := range ve {
			msg := humanizeFieldError(fe)
			if !seen[msg] {
				seen[msg] = true
				messages = append(messages, msg)
			}
		}
		return strings.Join(messages, " ")
	}

	// Malformed JSON, wrong field types, etc. — not a per-field validation error.
	return "Please check that all required fields are filled in correctly and try again."
}

func humanizeFieldError(fe validator.FieldError) string {
	field := friendlyFieldName(fe.Field())

	switch fe.Tag() {
	case "required":
		return field + " is required."
	case "email":
		return "Please enter a valid " + strings.ToLower(field) + "."
	case "min":
		return field + " must be at least " + fe.Param() + " characters long."
	case "max":
		return field + " must be at most " + fe.Param() + " characters long."
	case "oneof":
		return field + " must be one of: " + fe.Param() + "."
	default:
		return field + " is invalid."
	}
}

func friendlyFieldName(name string) string {
	if friendly, ok := friendlyFieldNames[name]; ok {
		return friendly
	}
	return name
}
