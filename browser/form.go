package browser

import (
	"github.com/diggernaut/goquery"
	"github.com/diggernaut/surf/errors"
	"net/url"
	"strings"
)

// Submittable represents an element that may be submitted, such as a form.
type Submittable interface {
	Method() string
	Action() string
	Input(name, value string) error
	Click(button string) error
	Submit(noclick string) error
	Dom() *goquery.Selection
}

// Form is the default form element.
type Form struct {
	bow         Browsable
	selection   *goquery.Selection
	method      string
	action      string
	fields      []*Field
    checkboxes  []*Checkbox
	buttons     []*Button
}
type Checkbox struct {
	name        string
	value       string
    checked     bool
}
type Field struct {
	name        string
	value       string
}
type Button struct {
	name        string
	value       string
}


// NewForm creates and returns a *Form type.
func NewForm(bow Browsable, s *goquery.Selection) *Form {
	fields, checkboxes, buttons := serializeForm(s)
	method, action := formAttributes(bow, s)

	return &Form{
		bow:        bow,
		selection:  s,
		method:     method,
        action:     action,
		fields:     fields,
		checkboxes: checkboxes,
		buttons:    buttons,
	}
}

// Method returns the form method, eg "GET" or "POST".
func (f *Form) Method() string {
	return f.method
}

// Action returns the form action URL.
// The URL will always be absolute.
func (f *Form) Action() string {
	return f.action
}

// Input sets the value of a form field.
func (f *Form) Input(name, value string) error {
    found := false
    for _, f := range f.fields {
        if f.name == name {
            f.value = value
            found = true
        }
    }
    for _, c := range f.checkboxes {
        if c.name == name && c.value == value {
            if c.checked {
                c.checked = false
            } else {
                c.checked = true
            }
            found = true
        }
    }
	if found {
		return nil
	}
	return errors.NewElementNotFound(
		"No input found with name '%s'.", name)
}

// Submit submits the form.
// Clicks the first button in the form, or submits the form without using
// any button when the form does not contain any buttons.
func (f *Form) Submit(noclick string) error {
	if noclick != "noclick" && len(f.buttons) > 0 {
		for _, b := range f.buttons {
			return f.Click(b.name)
		}
	}
	return f.send("", "")
}

// Click submits the form by clicking the button with the given name.
func (f *Form) Click(button string) error {
    found := false
    button_name := ""
    button_value := ""
    for _, b := range f.buttons {
        if b.name == button {
            found = true
            button_name = b.name
            button_value = b.value
            break
        }
    }
	if !found {
		return errors.NewInvalidFormValue(
			"Form does not contain a button with the name '%s'.", button)
	}
	return f.send(button_name, button_value)
}

// Dom returns the inner *goquery.Selection.
func (f *Form) Dom() *goquery.Selection {
	return f.selection
}

// send submits the form.
func (f *Form) send(buttonName, buttonValue string) error {
	method, ok := f.selection.Attr("method")
	if !ok {
		method = "GET"
	}
	action, ok := f.selection.Attr("action")
	if !ok {
		action = f.bow.Url().String()
	}
	aurl, err := url.Parse(action)
	if err != nil {
		return err
	}
	aurl = f.bow.ResolveUrl(aurl)

	values := make(url.Values)
	for _, field := range f.fields {
		values.Add(field.name, field.value)
	}
	for _, field := range f.checkboxes {
        if field.checked {
		    values.Add(field.name, field.value)
        }
	}
	if buttonName != "" {
		values.Add(buttonName, buttonValue)
	}

	if strings.ToUpper(method) == "GET" {
		return f.bow.OpenForm(aurl.String(), values)
	} else {
		enctype, _ := f.selection.Attr("enctype")
		if enctype == "multipart/form-data" {
			return f.bow.PostMultipart(aurl.String(), values, nil)
		}
		return f.bow.PostForm(aurl.String(), values, nil)
	}

	return nil
}

// Serialize converts the form fields into a url.Values type.
// Returns two url.Value types. The first is the form field values, and the
// second is the form button values.
func serializeForm(sel *goquery.Selection) ([]*Field, []*Checkbox, []*Button) {
	var fields []*Field
	var checkboxes []*Checkbox
	var buttons []*Button

	input := sel.Find("input,button,textarea,select")
	if input.Length() > 0 {
        input.Each(func(_ int, s *goquery.Selection) {
            name, ok := s.Attr("name")
            if ok {
                typ, ok := s.Attr("type")
                if s.Is("input") && ok || s.Is("textarea") {
                    if typ == "submit" {
                        val, ok := s.Attr("value")
                        if !ok {
                            val = ""
                        }
                        buttons = append(buttons, &Button{
                            name: name,
                            value: val,
                        })
                    } else if typ == "radio" {
                        val, ok := s.Attr("value")
                        if !ok {
                            val = ""
                        }
                        _, ok = s.Attr("checked")
                        if !ok {
                            val = ""
                        }
                        if val != "" {
                            fields = append(fields, &Field{
                                name: name,
                                value: val,
                            })
                        }
                    } else if typ == "checkbox" { 
                        val, ok := s.Attr("value")
                        if !ok {
                            val = ""
                        }
                        checked := true
                        _, ok = s.Attr("checked")
                        if !ok {
                            checked = false
                        }
                        checkboxes = append(checkboxes, &Checkbox{
                            name: name,
                            value: val,
                            checked: checked,
                        })
                    } else {
                        val, ok := s.Attr("value")
                        if !ok {
                            val = ""
                        }
                        fields = append(fields, &Field{
                            name: name,
                            value: val,
                        })
                    }
                } else if s.Is("select") {
                    options := s.Find("option")
                    val := ""
                    options.Each(func(idx int, option *goquery.Selection) {
                        _, ok := option.Attr("selected")
                        if idx == 0 || ok {
                            value, ok := option.Attr("value")
                            if ok {
                                val = value
                            }
                        }
                    })
                    fields = append(fields, &Field{
                        name: name,
                        value: val,
                    })
                }
            }
        })
	}
	return fields, checkboxes, buttons
}

func formAttributes(bow Browsable, s *goquery.Selection) (string, string) {
	method, ok := s.Attr("method")
	if !ok {
		method = "GET"
	}
	action, ok := s.Attr("action")
	if !ok {
		action = bow.Url().String()
	}
	aurl, err := url.Parse(action)
	if err != nil {
		return "", ""
	}
	aurl = bow.ResolveUrl(aurl)

	return strings.ToUpper(method), aurl.String()
}
