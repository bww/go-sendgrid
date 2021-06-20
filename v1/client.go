package sendgrid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/bww/go-util/v1/debug"
	"github.com/bww/go-util/v1/text"
	"github.com/bww/go-util/v1/urls"
)

const defaultEndpoint = "https://api.sendgrid.com/v3"

// A Sendgrid client
type Client interface {
	StoreContacts(contacts []*Contact, lists []string) error
	FetchContact(id string) (*Contact, error)
	FetchContactByEmail(email string) (*Contact, error)
	FetchContactWithParams(params url.Values) (*Contact, error)
	SendEmail(email *Email) error
}

type client struct {
	client          *http.Client
	base            string
	apikey          string
	overrideAddress string
	defaultSender   Address
	verbose         bool
}

// Create a client
func New(apikey string, opts ...Option) (Client, error) {
	conf := Config{
		Endpoint: defaultEndpoint,
		Verbose:  debug.VERBOSE || debug.DEBUG,
	}
	for _, o := range opts {
		conf = o(conf)
	}
	return &client{
		client:          &http.Client{Timeout: time.Second * 30},
		apikey:          apikey,
		base:            conf.Endpoint,
		overrideAddress: conf.OverrideAddress,
		defaultSender:   conf.DefaultSender,
		verbose:         conf.Verbose,
	}, nil
}

// Import contacts request
type storeContactsRequest struct {
	Lists    []string   `json:"list_ids"`
	Contacts []*Contact `json:"contacts"`
}

// Create or update a contact
func (c client) StoreContacts(contacts []*Contact, lists []string) error {
	entity := storeContactsRequest{
		Contacts: contacts,
		Lists:    lists,
	}

	data, err := json.Marshal(entity)
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
func (c client) FetchContactWithParams(params url.Values) (*Contact, error) {
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
func (c client) FetchContact(id string) (*Contact, error) {
	params := make(url.Values)
	params.Set("user_id", id)
	return c.FetchContactWithParams(params)
}

// Fetch a contact by their email address.
func (c client) FetchContactByEmail(email string) (*Contact, error) {
	params := make(url.Values)
	params.Set("email", email)
	return c.FetchContactWithParams(params)
}

// Send an email
func (c client) SendEmail(email *Email) error {
	data, err := json.Marshal(prepareEmail(email, c.defaultSender, c.overrideAddress))
	if err != nil {
		return err
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
func (c client) Send(req *http.Request) (*http.Response, []byte, error) {
	if c.apikey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apikey))
	}
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.verbose {
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

	if c.verbose {
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

func prepareEmail(email *Email, defaultSender Address, overrideAddress string) *Email {
	dup := *email
	if dup.From.IsZero() {
		dup.From = defaultSender
	}
	if dup.ReplyTo.IsZero() {
		dup.ReplyTo = defaultSender
	}
	if overrideAddress != "" {
		for _, p := range dup.Personalizations {
			for i, a := range p.Recipients {
				p.Recipients[i] = Address{
					Email: overrideAddress,
					Name:  a.Name,
				}
			}
		}
	}
	return &dup
}
