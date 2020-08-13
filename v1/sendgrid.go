package sendgrid

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bww/go-util/v1/debug"
	"github.com/bww/go-util/v1/text"
	"github.com/bww/go-util/v1/urls"
)

var (
	ErrBadRequest   = fmt.Errorf("Bad request")
	ErrForbidden    = fmt.Errorf("Forbidden")
	ErrUnauthorized = fmt.Errorf("Unauthorized")
	ErrServiceError = fmt.Errorf("Service error")
	ErrNotFound     = fmt.Errorf("Not found")
)

const defaultBase = "https://api.sendgrid.com/v3"

type Substitutions map[string]string

type Address struct {
	Email string `json:"email"`
	Name  string `json:"name"`
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

// A custom field
type Field struct {
	Id    int         `json:"id"`
	Name  string      `json:"name,omitempty"`
	Type  string      `json:"type,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

func Traits(m map[string]interface{}) []*Field {
	t := make([]*Field, 0, len(m))
	for k, v := range m {
		t = append(t, &Field{
			Name:  k,
			Type:  "string",
			Value: fmt.Sprint(v),
		})
	}
	return t
}

// A contact
type Contact struct {
	Id        string   `json:"id,omitempty"`
	Email     string   `json:"email,omitempty"`
	FirstName string   `json:"first_name,omitempty"`
	LastName  string   `json:"last_name,omitempty"`
	Lists     []string `json:"list_ids,omitempty"`
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

type Config struct {
	APIKey          string
	BaseURL         string
	OverrideAddress string
	DefaultSender   Address
	Simulate        bool
}

// A Sendgrid client
type Client struct {
	client          *http.Client
	base            string
	apikey          string
	overrideAddress string
	defaultSender   Address
	simulate        bool
}

// Create a client
func New(conf Config) (*Client, error) {
	var base string
	if conf.BaseURL != "" {
		base = conf.BaseURL
	} else {
		base = defaultBase
	}
	return &Client{
		client:          &http.Client{Timeout: time.Second * 30},
		base:            base,
		apikey:          conf.APIKey,
		overrideAddress: conf.OverrideAddress,
		defaultSender:   conf.DefaultSender,
		simulate:        conf.Simulate,
	}, nil
}

// Rebase
func (c Client) WithBaseURL(u string) *Client {
	d := c
	d.base = u
	return &d
}

// Default sender
func (c Client) DefaultSender() Address {
	return c.defaultSender
}

// Import contacts request
type storeContactsRequest struct {
	Lists    []string   `json:"list_ids"`
	Contacts []*Contact `json:"contacts"`
}

// Create or update a contact
func (c Client) StoreContacts(contacts []*Contact, lists []string) error {
	if c.simulate {
		return nil
	}

	data, err := json.Marshal(storeContactsRequest{Contacts: contacts, Lists: lists})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", urls.Join(c.base, "/marketing/contacts"), bytes.NewReader(data))
	if err != nil {
		return err
	}

	_, data, err = c.Send(req)
	if err != nil {
		return err
	}

	return nil
}

// Fetch a contact by their local identifier
func (c Client) fetchContact(params url.Values) (*Contact, error) {
	if c.simulate {
		return nil, ErrNotFound
	}

	u := urls.Join(c.base, "/marketing/contacts/search")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s?%s", u, params.Encode()), nil)
	if err != nil {
		return nil, err
	}

	_, data, err := c.Send(req)
	if err != nil {
		return nil, err
	}

	res := &struct {
		Contacts []*Contact `json:"result"`
	}{}

	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	} else if l := len(res.Contacts); l != 1 {
		return nil, ErrNotFound
	}

	return res.Contacts[0], nil
}

// Fetch a contact by their local identifier
func (c Client) FetchContact(id string) (*Contact, error) {
	params := make(url.Values)
	params.Set("ext_id", id)
	return c.fetchContact(params)
}

// Fetch a contact by their email address.
func (c Client) FetchContactByEmail(email string) (*Contact, error) {
	params := make(url.Values)
	params.Set("email", email)
	return c.fetchContact(params)
}

// Send an email
func (c Client) SendEmail(email *Email) error {
	var err error

	if c.overrideAddress != "" {
		for _, p := range email.Personalizations {
			for i, a := range p.Recipients {
				p.Recipients[i] = Address{
					Email: c.overrideAddress,
					Name:  a.Name,
				}
			}
		}
	}

	var data []byte
	if c.simulate {
		data, err = json.MarshalIndent(email, "", "  ")
	} else {
		data, err = json.Marshal(email)
	}
	if err != nil {
		return err
	}

	if c.simulate {
		fmt.Printf("sendgrid: POST %s\n", urls.Join(c.base, "/mail/send"))
		fmt.Println(text.Indent(string(data), "        > "))
		return nil
	}

	req, err := http.NewRequest("POST", urls.Join(c.base, "/mail/send"), bytes.NewReader(data))
	if err != nil {
		return err
	}

	_, _, err = c.Send(req)
	if err != nil {
		return err
	}

	return nil
}

// Perform an authenticated request; the parameter request will be
// mutated to include authentication and content type
func (c Client) Send(req *http.Request) (*http.Response, []byte, error) {
	if c.apikey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apikey))
	}
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if debug.VERBOSE {
		fmt.Println("sendgrid:", req.Method, req.URL)
		if req.Body != nil {
			data, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return nil, nil, err
			}
			req.Body = ioutil.NopCloser(bytes.NewBuffer(data))
			fmt.Println(text.Indent(string(data), " > "))
			fmt.Println(" * ")
		}
	}

	rsp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	} else {
		defer rsp.Body.Close()
	}

	data, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, nil, err
	}

	if debug.VERBOSE {
		fmt.Println(text.Indent(string(data), " < "))
		fmt.Println()
	}

	if rsp.StatusCode >= 200 && rsp.StatusCode < 300 {
		return rsp, data, nil
	}

	switch rsp.StatusCode {
	case http.StatusForbidden:
		return nil, nil, ErrForbidden
	case http.StatusUnauthorized:
		return nil, nil, ErrUnauthorized
	case http.StatusBadRequest:
		return nil, nil, ErrBadRequest
	case http.StatusInternalServerError:
		return nil, nil, ErrServiceError
	default:
		return nil, nil, fmt.Errorf("Unexpected status code: %v", rsp.Status)
	}
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
