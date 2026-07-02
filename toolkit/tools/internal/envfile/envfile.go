// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// Used to parse config files formatted like a Bash script file containing only variable assignments.

package envfile

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/file"
	"github.com/microsoft/azure-linux-image-tools/toolkit/tools/internal/grub"
)

type NameValuePair struct {
	Name  string
	Value string
}

func ParseEnvFile(path string) (map[string]string, error) {
	content, err := file.Read(path)
	if err != nil {
		return nil, err
	}

	return ParseEnv(content)
}

func ParseEnv(content string) (map[string]string, error) {
	result := make(map[string]string)

	err := parseEnvHelper(content, func(name, value string) {
		result[name] = value
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func ParseEnvList(content string) ([]struct{ Name, Value string }, error) {
	result := []struct{ Name, Value string }(nil)

	err := parseEnvHelper(content, func(name, value string) {
		result = append(result, NameValuePair{Name: name, Value: value})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func parseEnvHelper(content string, yield func(name, value string)) error {
	tokens, err := grub.TokenizeConfig(content)
	if err != nil {
		return err
	}

	lines := grub.SplitTokensIntoLines(tokens)
	for _, line := range lines {
		if len(line.Tokens) > 2 {
			loc := line.Tokens[1].Loc.Start
			return fmt.Errorf("env file line has multiple words (%d:%d)", loc.Line, loc.Col)
		}

		token := line.Tokens[0]

		// Variable assignments can not have any character escaping before the '=' char.
		if token.Type != grub.WORD &&
			token.SubWords[0].Type != grub.KEYWORD_STRING {
			loc := token.Loc.Start
			return fmt.Errorf("env file line is not a variable assignment (%d:%d)", loc.Line, loc.Col)
		}

		firstWord := token.SubWords[0].Value

		// Find the '=' char.
		eqIndex := strings.Index(firstWord, "=")
		if eqIndex < 0 {
			loc := token.Loc.Start
			return fmt.Errorf("env file line is not a variable assignment (%d:%d)", loc.Line, loc.Col)
		}

		name := firstWord[:eqIndex]

		valueBuilder := strings.Builder{}
		valueBuilder.WriteString(firstWord[eqIndex+1:])

		for _, word := range token.SubWords[1:] {
			switch word.Type {
			case grub.KEYWORD_STRING, grub.STRING:
				valueBuilder.WriteString(word.Value)

			default:
				loc := word.Loc.Start
				return fmt.Errorf("env file contains invalid characters (%d:%d)", loc.Line, loc.Col)
			}
		}

		value := valueBuilder.String()

		yield(name, value)
	}

	return nil
}
