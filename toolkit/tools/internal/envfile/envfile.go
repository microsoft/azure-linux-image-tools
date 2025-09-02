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

func ParseEnvFile(path string) (map[string]string, error) {
	content, err := file.Read(path)
	if err != nil {
		return nil, err
	}

	return ParseEnv(content)
}

func ParseEnv(content string) (map[string]string, error) {
	tokens, err := grub.TokenizeConfig(content)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)

	lines := grub.SplitTokensIntoLines(tokens)
	for _, line := range lines {
		if len(line.Tokens) > 2 {
			loc := line.Tokens[1].Loc.Start
			return nil, fmt.Errorf("env file line has multiple words (%d:%d)", loc.Line, loc.Col)
		}

		token := line.Tokens[0]

		// Variable assignments can not have any character escaping before the '=' char.
		if token.Type != grub.WORD &&
			token.SubWords[0].Type != grub.KEYWORD_STRING {
			loc := token.Loc.Start
			return nil, fmt.Errorf("env file line is not a variable assignment (%d:%d)", loc.Line, loc.Col)
		}

		firstWord := token.SubWords[0].Value

		// Find the '=' char.
		eqIndex := strings.Index(firstWord, "=")
		if eqIndex < 0 {
			loc := token.Loc.Start
			return nil, fmt.Errorf("env file line is not a variable assignment (%d:%d)", loc.Line, loc.Col)
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
				return nil, fmt.Errorf("env file contains invalid characters (%d:%d)", loc.Line, loc.Col)
			}
		}

		value := valueBuilder.String()

		result[name] = value
	}

	return result, nil
}
