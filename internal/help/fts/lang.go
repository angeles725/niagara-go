package fts

import "strings"

// stopWords is search_lang.STOP_WORDS: English and Spanish words a query
// drops unless it is nothing else (a test compares it with the Python set).
var stopWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(stopWordText) {
		m[w] = true
	}
	return m
}()

const stopWordText = `
a an and are as at be been but by can could do does did for from has have
how i if in into is it its may me my no not of on or our should so than that
the their them then there these they this those to too use used using vs was
we what when where which who why will with would you your about also any all
each other such via
al como con cual cuales cuando de del donde el en entre es esta este esto
hay la las lo los mas me mi mis o para pero por que se si sin sobre son su
sus tiene un una uno unos unas y ya
`
