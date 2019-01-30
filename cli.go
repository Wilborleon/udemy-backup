package main

import (
	"errors"

	"github.com/manifoldco/promptui"
	"github.com/ushu/udemy-backup/client"
)

// selectCourse allows to select a course among a previously-downloaded list
func selectCourse(courses []*client.Course) (*client.Course, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "🤓 {{ .Title | cyan }} ({{ .ID | red }})",
		Inactive: "  {{ .Title | cyan }} ({{ .ID | red }})",
		Selected: "🚀 {{ .Title | red | cyan }}",
		Details: `
--------- Course ----------
{{ "Title:" | faint }}	{{ .Title }}
{{ "URL:" | faint }}	https://www.udemy.com{{ .URL }}`,
	}

	prompt := promptui.Select{
		Label:     "Select course to backup",
		Items:     courses,
		Templates: templates,
	}

	i, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}
	return courses[i], nil
}

func askCredentials() (login string, password string, err error) {
	prompt := promptui.Prompt{
		Label:    "Email",
		Validate: notEmpty,
	}
	login, err = prompt.Run()
	if err != nil {
		return
	}

	prompt = promptui.Prompt{
		Label:    "Password",
		Mask:     '•',
		Validate: notEmpty,
	}
	password, err = prompt.Run()
	return
}

func notEmpty(s string) error {
	if s == "" {
		return errors.New("cannot be empty")
	}
	return nil
}