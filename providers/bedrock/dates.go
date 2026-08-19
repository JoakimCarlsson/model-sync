package bedrock

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// months name the point in the year AWS writes a date with, in both the
// shortened and the written-out form its pages mix.
var months = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var (
	// namedDayRe matches a date written as a month, a day and a year, which
	// is how a card dates a launch and how the lifecycle page dates a
	// retirement.
	namedDayRe = regexp.MustCompile(
		`(?i)\b([a-z]{3,9})\.?\s+(\d{1,2}),\s*(\d{4})\b`,
	)
	// namedMonthRe matches a date written as a month and a year alone, which
	// is how a card dates a knowledge cutoff.
	namedMonthRe = regexp.MustCompile(`(?i)\b([a-z]{3,9})\.?\s+(\d{4})\b`)
	// numericRe matches a date written in figures, which is how a card dates
	// the end of life it promises not to come before.
	numericRe = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{4})\b`)
)

// isoDate rewrites a date as AWS's pages state it into the form the catalog
// keeps dates in, to the precision published and no further: a month and a
// year stay a month and a year.
//
// A value stating no date at all comes back empty, which is what a card's
// "N/A" end of life amounts to. The wording around a date is dropped, since
// an end of life AWS will not come before is still that date.
func isoDate(value string) string {
	if match := namedDayRe.FindStringSubmatch(value); match != nil {
		if month, ok := months[monthKey(match[1])]; ok {
			return fmt.Sprintf(
				"%s-%02d-%02d",
				match[3],
				month,
				number(match[2]),
			)
		}
	}
	if match := numericRe.FindStringSubmatch(value); match != nil {
		return fmt.Sprintf(
			"%s-%02d-%02d",
			match[3],
			number(match[1]),
			number(match[2]),
		)
	}
	if match := namedMonthRe.FindStringSubmatch(value); match != nil {
		if month, ok := months[monthKey(match[1])]; ok {
			return fmt.Sprintf("%s-%02d", match[2], month)
		}
	}
	return ""
}

// monthKey reduces a month to the three letters both spellings share.
func monthKey(name string) string {
	name = strings.ToLower(name)
	if len(name) < 3 {
		return name
	}
	return name[:3]
}

// number reads a figure of a date.
func number(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
