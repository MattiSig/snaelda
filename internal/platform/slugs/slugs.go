package slugs

import (
	"regexp"
	"strings"
)

const (
	DefaultFallback = "untitled"
	MaxLength       = 80
)

var validPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func Generate(value string) string {
	return GenerateWithFallback(value, DefaultFallback)
}

func GenerateWithFallback(value string, fallback string) string {
	slug := normalize(value)
	if slug == "" {
		slug = normalize(fallback)
	}
	if slug == "" {
		slug = DefaultFallback
	}

	if len(slug) > MaxLength {
		slug = strings.Trim(slug[:MaxLength], "-")
	}
	if slug == "" {
		return DefaultFallback
	}
	return slug
}

func EnsureUnique(base string, exists func(string) (bool, error)) (string, error) {
	candidate := Generate(base)
	for suffix := 2; ; suffix++ {
		taken, err := exists(candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}

		suffixText := "-" + itoa(suffix)
		trimmedBase := strings.Trim(candidateBase(base), "-")
		if len(trimmedBase)+len(suffixText) > MaxLength {
			trimmedBase = strings.Trim(trimmedBase[:MaxLength-len(suffixText)], "-")
		}
		if trimmedBase == "" {
			trimmedBase = DefaultFallback
		}
		candidate = trimmedBase + suffixText
	}
}

func IsValid(value string) bool {
	return validPattern.MatchString(value)
}

func PagePath(title string) string {
	if strings.TrimSpace(title) == "" {
		return "/"
	}

	slug := Generate(title)
	if slug == "home" || slug == "index" {
		return "/"
	}
	return "/" + slug
}

func normalize(value string) string {
	var builder strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if replacement, ok := runeReplacement(r); ok {
			for _, rr := range replacement {
				writeSlugRune(&builder, rr, &lastDash)
			}
			continue
		}
		writeSlugRune(&builder, r, &lastDash)
	}

	return strings.Trim(builder.String(), "-")
}

func writeSlugRune(builder *strings.Builder, r rune, lastDash *bool) {
	if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
		builder.WriteRune(r)
		*lastDash = false
		return
	}
	if !*lastDash {
		builder.WriteRune('-')
		*lastDash = true
	}
}

func candidateBase(value string) string {
	return Generate(value)
}

func runeReplacement(r rune) (string, bool) {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å', 'ā':
		return "a", true
	case 'æ':
		return "ae", true
	case 'ç', 'č':
		return "c", true
	case 'ð', 'ď':
		return "d", true
	case 'é', 'è', 'ê', 'ë', 'ē':
		return "e", true
	case 'í', 'ì', 'î', 'ï', 'ī':
		return "i", true
	case 'ñ':
		return "n", true
	case 'ó', 'ò', 'ô', 'ö', 'õ', 'ø', 'ō':
		return "o", true
	case 'þ':
		return "th", true
	case 'ú', 'ù', 'û', 'ü', 'ū':
		return "u", true
	case 'ý', 'ÿ':
		return "y", true
	case 'ß':
		return "ss", true
	default:
		return "", false
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
