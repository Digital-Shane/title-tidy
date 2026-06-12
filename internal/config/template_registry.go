package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
)

const (
	escapedLeftBraceToken  = "\x00title-tidy-left-brace\x00"
	escapedRightBraceToken = "\x00title-tidy-right-brace\x00"
)

var (
	templateEscapeReplacer  = strings.NewReplacer(`\{`, escapedLeftBraceToken, `\}`, escapedRightBraceToken)
	templateRestoreReplacer = strings.NewReplacer(escapedLeftBraceToken, "{", escapedRightBraceToken, "}")
)

// TemplateVariablesForMediaType returns provider-owned variables available for
// a media type, using provider display order when multiple providers supply the
// same variable.
func TemplateVariablesForMediaType(reg *provider.Registry, mediaType provider.MediaType) []provider.TemplateVariable {
	result := []provider.TemplateVariable{}
	for _, definitions := range templateVariablesByName(reg) {
		if variable, ok := primaryVariableForMediaType(definitions, mediaType); ok {
			result = append(result, variable)
		}
	}
	return result
}

// TemplateVariableProviders returns providers that can supply the variable.
func TemplateVariableProviders(reg *provider.Registry, variableName string) []provider.Provider {
	reg = providerRegistryOrGlobal(reg)
	providers := make([]provider.Provider, 0)
	for _, p := range ProvidersByDisplayOrder(reg) {
		if p == nil {
			continue
		}
		for _, v := range p.SupportedVariables() {
			if v.Name == variableName {
				providers = append(providers, p)
				break
			}
		}
	}
	return providers
}

// TemplateVariableDisplayOrder returns the earliest provider display order for
// a variable, or a large default when no provider owns it.
func TemplateVariableDisplayOrder(reg *provider.Registry, variableName string) int {
	priority := 1_000
	for _, p := range TemplateVariableProviders(reg, variableName) {
		if order := p.Capabilities().DisplayOrder; order < priority {
			priority = order
		}
	}
	return priority
}

// ResolveTemplate processes a template string with metadata.
func ResolveTemplate(template string, ctx *FormatContext, metadata *provider.Metadata, reg *provider.Registry) (string, error) {
	return resolveTemplate(template, ctx, metadata, templateVariablesByName(reg))
}

func templateVariablesByName(reg *provider.Registry) map[string][]provider.TemplateVariable {
	variables := make(map[string][]provider.TemplateVariable)
	for _, p := range ProvidersByDisplayOrder(reg) {
		if p == nil {
			continue
		}
		name := p.Name()
		for _, v := range p.SupportedVariables() {
			v.Provider = name
			variables[v.Name] = append(variables[v.Name], v)
		}
	}
	return variables
}

func primaryVariableForMediaType(definitions []provider.TemplateVariable, mediaType provider.MediaType) (provider.TemplateVariable, bool) {
	for _, variable := range definitions {
		if templateVariableSupportsMediaType(variable, mediaType) {
			return variable, true
		}
	}
	return provider.TemplateVariable{}, false
}

var templateVariablePattern = regexp.MustCompile(`\{([^}]+)\}`)

func resolveTemplate(template string, ctx *FormatContext, metadata *provider.Metadata, variables map[string][]provider.TemplateVariable) (string, error) {
	escapedTemplate := escapeTemplateBraces(template)
	result := escapedTemplate
	if variables == nil {
		variables = map[string][]provider.TemplateVariable{}
	}

	matches := templateVariablePattern.FindAllStringSubmatch(escapedTemplate, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		varName := match[1]
		varPlaceholder := match[0] // includes the braces

		value, err := resolveVariable(varName, ctx, metadata, variables)
		if err != nil {
			value = ""
		}

		result = strings.ReplaceAll(result, varPlaceholder, value)
	}

	result = restoreTemplateBraces(result)

	// Clean up the result
	result = local.CleanName(result)

	return result, nil
}

func escapeTemplateBraces(template string) string {
	return templateEscapeReplacer.Replace(template)
}

func restoreTemplateBraces(template string) string {
	return templateRestoreReplacer.Replace(template)
}

func resolveVariable(varName string, ctx *FormatContext, metadata *provider.Metadata, variables map[string][]provider.TemplateVariable) (string, error) {
	definitions := variables[varName]
	if len(definitions) == 0 {
		return "", fmt.Errorf("variable %s not found", varName)
	}

	for _, variable := range definitions {
		if variable.ValueSource == provider.TemplateVariableValueSourceContext {
			continue
		}
		if value, ok := provider.ResolveMetadataVariable(variable, metadata); ok {
			return value, nil
		}
	}

	for _, variable := range definitions {
		if variable.ValueSource != provider.TemplateVariableValueSourceContext {
			continue
		}
		if value := resolveContextVariable(variable, ctx); value != "" {
			return value, nil
		}
	}

	return "", fmt.Errorf("variable %s not found", varName)
}

func resolveContextVariable(variable provider.TemplateVariable, ctx *FormatContext) string {
	if ctx == nil {
		return ""
	}
	key := variable.ValueKey
	if key == "" {
		key = variable.Name
	}
	switch key {
	case provider.TemplateValueKeyTitle:
		if ctx.MovieName != "" {
			return ctx.MovieName
		}
		return ctx.ShowName
	case provider.TemplateValueKeyYear:
		return ctx.Year
	case provider.TemplateValueKeySeason:
		if ctx.Season >= 0 {
			return fmt.Sprintf("%02d", ctx.Season)
		}
		return ""
	case provider.TemplateValueKeyEpisode:
		if ctx.Episode >= 0 {
			return fmt.Sprintf("%02d", ctx.Episode)
		}
		return ""
	default:
		return ""
	}
}

func templateVariableSupportsMediaType(variable provider.TemplateVariable, mediaType provider.MediaType) bool {
	if len(variable.MediaTypes) == 0 {
		return true
	}
	for _, mt := range variable.MediaTypes {
		if mt == mediaType {
			return true
		}
	}
	return false
}
