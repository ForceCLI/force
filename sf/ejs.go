package sf

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// ejsScope holds the variables of a template of the kind the flexipage
// templates of sf are: text values and lists of text values.
type ejsScope struct {
	values map[string]string
	lists  map[string][]string
}

func (s *ejsScope) child() *ejsScope {
	c := &ejsScope{values: map[string]string{}, lists: s.lists}
	for key, value := range s.values {
		c.values[key] = value
	}
	return c
}

var (
	ejsTag        = regexp.MustCompile(`(?s)<%(=?)(.*?)%>`)
	ejsIf         = regexp.MustCompile(`^if \((.*)\) \{$`)
	ejsForEach    = regexp.MustCompile(`^(\w+)\.forEach\(\((\w+), (\w+)\) => \{$`)
	ejsReplaceDot = regexp.MustCompile(`^(\w+)\.replace\(/\\\./g, '_'\)$`)
	ejsLength     = regexp.MustCompile(`^(\w+)\.length > 0$`)
)

// ejsEscape escapes a value the way EJS does for <%= %>.
func ejsEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(value)
}

func (s *ejsScope) truthy(condition string) (bool, error) {
	for _, part := range strings.Split(condition, " && ") {
		part = strings.TrimSpace(part)
		if match := ejsLength.FindStringSubmatch(part); match != nil {
			if len(s.lists[match[1]]) == 0 {
				return false, nil
			}
			continue
		}
		if list, isList := s.lists[part]; isList {
			// An array is true in JavaScript, also when it is empty.
			_ = list
			continue
		}
		value, known := s.values[part]
		if !known {
			return false, fmt.Errorf("template condition %q is not supported", condition)
		}
		if value == "" {
			return false, nil
		}
	}
	return true, nil
}

func (s *ejsScope) expression(text string) (string, error) {
	text = strings.TrimSpace(text)
	if match := ejsReplaceDot.FindStringSubmatch(text); match != nil {
		return strings.ReplaceAll(s.values[match[1]], ".", "_"), nil
	}
	if value, known := s.values[text]; known {
		return value, nil
	}
	return "", fmt.Errorf("template expression %q is not supported", text)
}

// renderEJS renders a template that uses the subset of EJS that the
// flexipage templates of sf use: output tags with a variable or a
// replacement of periods, if blocks, forEach loops over a list, and the
// block that splits the detail fields into two columns.
func renderEJS(template string, scope *ejsScope) (string, error) {
	type token struct {
		text   string
		code   string
		output bool
		isCode bool
	}
	var tokens []token
	last := 0
	for _, match := range ejsTag.FindAllStringSubmatchIndex(template, -1) {
		tokens = append(tokens, token{text: template[last:match[0]]})
		tokens = append(tokens, token{code: strings.TrimSpace(template[match[4]:match[5]]), output: template[match[2]:match[3]] == "=", isCode: true})
		last = match[1]
	}
	tokens = append(tokens, token{text: template[last:]})

	var render func(position int, scope *ejsScope, active bool) (string, int, error)
	render = func(position int, scope *ejsScope, active bool) (string, int, error) {
		var b strings.Builder
		for position < len(tokens) {
			t := tokens[position]
			position++
			if !t.isCode {
				if active {
					b.WriteString(t.text)
				}
				continue
			}
			if t.output {
				if active {
					value, err := scope.expression(t.code)
					if err != nil {
						return "", 0, err
					}
					b.WriteString(ejsEscape(value))
				}
				continue
			}
			switch {
			case t.code == "}" || t.code == "});":
				return b.String(), position, nil
			case ejsIf.MatchString(t.code):
				condition := false
				if active {
					var err error
					condition, err = scope.truthy(ejsIf.FindStringSubmatch(t.code)[1])
					if err != nil {
						return "", 0, err
					}
				}
				body, next, err := render(position, scope, active && condition)
				if err != nil {
					return "", 0, err
				}
				b.WriteString(body)
				position = next
			case ejsForEach.MatchString(t.code):
				match := ejsForEach.FindStringSubmatch(t.code)
				items := scope.lists[match[1]]
				next := position
				if !active || len(items) == 0 {
					_, end, err := render(position, scope, false)
					if err != nil {
						return "", 0, err
					}
					next = end
				}
				for index, item := range items {
					if !active {
						break
					}
					inner := scope.child()
					inner.values[match[2]] = item
					inner.values[match[3]] = fmt.Sprintf("%d", index)
					body, end, err := render(position, inner, true)
					if err != nil {
						return "", 0, err
					}
					b.WriteString(body)
					next = end
				}
				position = next
			case strings.Contains(t.code, "allDetailFields"):
				// The detail fields, or four default fields, split into
				// two columns of which the first is the longer one.
				fields := scope.lists["detailFields"]
				if len(fields) == 0 {
					fields = []string{"Name", "OwnerId", "CreatedById", "LastModifiedById"}
				}
				midpoint := int(math.Ceil(float64(len(fields)) / 2))
				scope.lists["column1Fields"] = fields[:midpoint]
				scope.lists["column2Fields"] = fields[midpoint:]
			default:
				return "", 0, fmt.Errorf("template statement %q is not supported", t.code)
			}
		}
		return b.String(), position, nil
	}
	rendered, _, err := render(0, scope, true)
	return rendered, err
}
