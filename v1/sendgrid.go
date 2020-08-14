package sendgrid

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/bww/go-util/v1/debug"
)

var (
	ErrBadRequest   = fmt.Errorf("Bad request")
	ErrForbidden    = fmt.Errorf("Forbidden")
	ErrUnauthorized = fmt.Errorf("Unauthorized")
	ErrServiceError = fmt.Errorf("Service error")
	ErrNotFound     = fmt.Errorf("Not found")
)

type Substitutions map[string]string

type Address struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (a Address) IsZero() bool {
	return a.Email == ""
}

type Personalization struct {
	Recipients    []Address     `json:"to"`
	Substitutions Substitutions `json:"dynamic_template_data,omitempty"`
	Subject       string        `json:"subject,omitempty"`
}

type Attachment struct {
	Content     string `json:"content"`
	Type        string `json:"type,omitempty"`
	Filename    string `json:"filename"`
	Disposition string `json:"disposition,omitempty"`
	ContentId   string `json:"content_id,omitempty"`
}

func NewAttachment(mtype, fname string, data []byte) *Attachment {
	return &Attachment{
		Content:  base64.StdEncoding.EncodeToString(data),
		Type:     mtype,
		Filename: fname,
	}
}

// A templated email
type Email struct {
	TemplateId       string            `json:"template_id"`
	From             Address           `json:"from"`
	ReplyTo          Address           `json:"reply_to"`
	Personalizations []Personalization `json:"personalizations"`
	Attachments      []*Attachment     `json:"attachments"`
}

// Custom fields
type Fields map[string]interface{}

// A contact
type Contact struct {
	Id        string   `json:"id,omitempty"`
	Email     string   `json:"email,omitempty"`
	FirstName string   `json:"first_name,omitempty"`
	LastName  string   `json:"last_name,omitempty"`
	Lists     []string `json:"list_ids,omitempty"`
	Fields    Fields   `json:"custom_fields,omitempty"`
}

// An error
type Error struct {
	Message string `json:"message"`
	Indices []int  `json:"error_indices,omitempty"`
}

func (e Error) Error() string {
	var s strings.Builder
	s.WriteString(e.Message)
	if debug.VERBOSE && len(e.Indices) > 0 {
		s.WriteString(" (input indices: ")
		for i, e := range e.Indices {
			if i > 0 {
				s.WriteString(", ")
			}
			s.WriteString(strconv.Itoa(e))
		}
		s.WriteString(")")
	}
	return s.String()
}

func SplitName(n string) (string, string) {
	var first, last string
	if i := strings.LastIndex(n, " "); i > 0 {
		first = strings.TrimSpace(n[:i])
		last = strings.TrimSpace(n[i+1:])
	} else {
		first = n
	}
	return first, last
}
