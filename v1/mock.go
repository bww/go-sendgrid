package sendgrid

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/bww/go-util/v1/text"
	"github.com/bww/go-util/v1/urls"
)

type mock struct {
	base            string
	defaultSender   Address
	overrideAddress string
	verbose         bool
}

func Mock(apikey string, opts ...Option) (Client, error) {
	conf := Config{
		Endpoint: defaultEndpoint,
	}
	for _, o := range opts {
		conf = o(conf)
	}
	return &mock{
		base:            conf.Endpoint,
		defaultSender:   conf.DefaultSender,
		overrideAddress: conf.OverrideAddress,
		verbose:         conf.Verbose,
	}, nil
}

func (c mock) SendEmail(email *Email) error {
	c.dump("POST", "/mail/send", prepareEmail(email, c.defaultSender, c.overrideAddress))
	return nil
}

func (c mock) StoreContacts(contacts []*Contact, lists []string) error {
	c.dump("PUT", "/marketing/contacts", storeContactsRequest{Contacts: contacts, Lists: lists})
	return nil
}

func (c mock) FetchContact(id string) (*Contact, error) {
	params := make(url.Values)
	params.Set("ext_id", id)
	return c.FetchContactWithParams(params)
}

func (c mock) FetchContactByEmail(email string) (*Contact, error) {
	params := make(url.Values)
	params.Set("email", email)
	return c.FetchContactWithParams(params)
}

func (c mock) FetchContactWithParams(params url.Values) (*Contact, error) {
	path := "/marketing/contacts/search"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	c.dump("POST", path, nil)
	return nil, ErrNotFound
}

func (c mock) dump(method, url string, entity interface{}) error {
	var data []byte
	if c.verbose && entity != nil {
		var err error
		data, err = json.MarshalIndent(entity, "", "  ")
		if err != nil {
			return err
		}
	}
	fmt.Printf("sendgrid: %s %s\n", method, urls.Join(c.base, url))
	if len(data) > 0 {
		fmt.Println(text.Indent(string(data), "        > "))
	}
	return nil
}
