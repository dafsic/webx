package i18n

import (
	"embed"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

const defaultLang = "en"
const i18nHeaderKey = "lang"

var bundle *i18n.Bundle

// InitI18n initializes the i18n bundle with all available translations
func InitI18n() error {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	//Load all translation files
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return fmt.Errorf("failed to read locales directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			_, err := bundle.LoadMessageFileFS(localeFS, "locales/"+entry.Name())
			if err != nil {
				return fmt.Errorf("failed to load message file %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// GetLangFromContext extracts the preferred language from the gin.Context
func GetLangFromContext(c *gin.Context) string {
	lang := c.GetHeader(i18nHeaderKey)
	if lang == "" {
		return defaultLang
	}
	// Extract language code if format is like "en-US"
	parts := strings.Split(lang, "-")
	return strings.ToLower(parts[0])
}

// T translates a message to the specified language
func T(lang, messageID string, templateData map[string]interface{}) string {
	localizer := i18n.NewLocalizer(bundle, lang, defaultLang)
	message, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
	if err != nil {
		return messageID // Return message ID as fallback
	}
	return message
}
