// Sentence segmentation over scrubbed prose. The splitter is deliberately
// conservative: requirement statements are usually plain declarative
// sentences, and a missed split merely widens the unit a rule looks at,
// while a false split can cut a keyword away from its requirement ID.
package spec

import "strings"

// hardAbbrev are tokens after which a period never ends a sentence,
// whatever follows. Kept short on purpose — see softStop below for the rest.
var hardAbbrev = map[string]bool{
	"e.g": true, "i.e": true, "vs": true, "cf": true, "et": true,
	"mr": true, "ms": true, "dr": true, "st": true,
}

// splitSentences returns [start, end) byte ranges of sentences in text.
// Newlines inside text are ordinary whitespace (blocks join their lines
// with \n), so sentences span source lines naturally.
func splitSentences(text string) [][2]int {
	var out [][2]int
	start := 0
	i := 0
	for i < len(text) {
		c := text[i]
		if c != '.' && c != '!' && c != '?' {
			i++
			continue
		}
		// Collapse runs like "..." or "?!" into one candidate terminator.
		j := i
		for j < len(text) && (text[j] == '.' || text[j] == '!' || text[j] == '?') {
			j++
		}
		if isSentenceEnd(text, i, j) {
			// Include trailing closers (quotes, brackets) in the sentence.
			j = absorbClosers(text, j)
			out = appendRange(out, text, start, j)
			start = j
			i = j
			continue
		}
		i = j
	}
	out = appendRange(out, text, start, len(text))
	return out
}

// absorbClosers extends the sentence end over closing quotes and brackets.
func absorbClosers(text string, j int) int {
	for j < len(text) {
		switch text[j] {
		case '"', '\'', ')', ']':
			j++
			continue
		}
		if strings.HasPrefix(text[j:], "”") || strings.HasPrefix(text[j:], "’") {
			j += 3
			continue
		}
		break
	}
	return j
}

// isSentenceEnd decides whether the punctuation run text[i:j] terminates a
// sentence.
func isSentenceEnd(text string, i, j int) bool {
	if text[i] == '.' && j-i == 1 {
		// "1.2" and "v0.1.0" — a digit on both sides is a version/number.
		if i > 0 && isDigit(text[i-1]) && j < len(text) && isDigit(text[j]) {
			return false
		}
		word := lastWord(text[:i])
		if hardAbbrev[strings.ToLower(word)] {
			return false
		}
		// Single-letter "words" are initials or enumerators ("a." "B.").
		if len(word) == 1 {
			return false
		}
	}
	// Peek past closers and whitespace at what starts next.
	k := absorbClosers(text, j)
	if k >= len(text) {
		return true
	}
	if !isSpace(text[k]) {
		return false // "etc.)," mid-token, "3.x", "example.test"
	}
	for k < len(text) && isSpace(text[k]) {
		k++
	}
	if k >= len(text) {
		return true
	}
	// softStop: a following lowercase letter means the sentence continues —
	// this cheaply handles "etc. and", "approx. two", "no. 5".
	c := text[k]
	if c >= 'a' && c <= 'z' {
		return false
	}
	return true
}

func appendRange(out [][2]int, text string, start, end int) [][2]int {
	s, e := trimRange(text, start, end)
	if s < e {
		out = append(out, [2]int{s, e})
	}
	return out
}

// trimRange shrinks [start, end) so it begins and ends on non-space bytes.
func trimRange(text string, start, end int) (int, int) {
	for start < end && isSpace(text[start]) {
		start++
	}
	for end > start && isSpace(text[end-1]) {
		end--
	}
	return start, end
}

func lastWord(s string) string {
	end := len(s)
	for end > 0 && isSpace(s[end-1]) {
		end--
	}
	start := end
	for start > 0 && !isSpace(s[start-1]) && s[start-1] != '(' && s[start-1] != '"' {
		start--
	}
	// Keep interior dots ("e.g") but drop other punctuation.
	return strings.Trim(s[start:end], ",;:()[]'\"")
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
