// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"regexp"
	"strings"
	"time"
)

// ParseICS parses RFC 5545 iCalendar (VCALENDAR / VEVENT) data into CalendarInvite records.
// Completely language-agnostic: relies only on standard RFC 5545 property names.
func ParseICS(raw string) []CalendarInvite {
	unfolded := unfoldICSLines(raw)
	lines := strings.Split(unfolded, "\n")

	var (
		invites      []CalendarInvite
		calMethod    string
		inEvent      bool
		cur          CalendarInvite
		startTZ      string
		endTZ        string
		rawStartTime string
		rawEndTime   string
	)

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if upper == "BEGIN:VEVENT" {
			inEvent = true
			cur = CalendarInvite{Method: calMethod}
			startTZ, endTZ, rawStartTime, rawEndTime = "", "", "", ""
			continue
		}
		if upper == "END:VEVENT" {
			if inEvent {
				cur.StartTime = parseICSTimestamp(rawStartTime, startTZ)
				cur.EndTime = parseICSTimestamp(rawEndTime, endTZ)
				if cur.EndTime.IsZero() && !cur.StartTime.IsZero() {
					cur.EndTime = cur.StartTime.Add(time.Hour)
				}
				if !cur.StartTime.IsZero() || cur.Summary != "" {
					invites = append(invites, cur)
				}
			}
			inEvent = false
			continue
		}

		name, params, val := splitICSProperty(line)
		if !inEvent {
			if name == "METHOD" {
				calMethod = strings.ToUpper(strings.TrimSpace(val))
			}
			continue
		}

		switch name {
		case "UID":
			cur.UID = strings.TrimSpace(val)
		case "SUMMARY":
			cur.Summary = unescapeICSText(val)
		case "DESCRIPTION":
			cur.Description = unescapeICSText(val)
		case "LOCATION":
			cur.Location = unescapeICSText(val)
		case "ORGANIZER":
			cur.Organizer = parseICSOrganizer(params, val)
		case "DTSTART":
			rawStartTime = strings.TrimSpace(val)
			startTZ = params["TZID"]
		case "DTEND":
			rawEndTime = strings.TrimSpace(val)
			endTZ = params["TZID"]
		}
	}

	return invites
}

func unfoldICSLines(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	var out strings.Builder
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if i > 0 && len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			out.WriteString(line[1:])
		} else {
			if i > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(line)
		}
	}
	return out.String()
}

func splitICSProperty(line string) (name string, params map[string]string, value string) {
	params = make(map[string]string)
	colonIdx := strings.IndexByte(line, ':')
	if colonIdx < 0 {
		return strings.ToUpper(line), params, ""
	}
	left := line[:colonIdx]
	value = line[colonIdx+1:]

	parts := strings.Split(left, ";")
	name = strings.ToUpper(strings.TrimSpace(parts[0]))
	for _, p := range parts[1:] {
		eq := strings.IndexByte(p, '=')
		if eq > 0 {
			k := strings.ToUpper(strings.TrimSpace(p[:eq]))
			v := strings.Trim(strings.TrimSpace(p[eq+1:]), `"`)
			params[k] = v
		}
	}
	return name, params, value
}

func parseICSTimestamp(val, tzid string) time.Time {
	val = strings.TrimSpace(val)
	if val == "" {
		return time.Time{}
	}
	loc := time.Local
	if tzid != "" {
		if l, err := time.LoadLocation(tzid); err == nil {
			loc = l
		}
	}
	if strings.HasSuffix(val, "Z") {
		if t, err := time.Parse("20060102T150405Z", val); err == nil {
			return t.In(loc)
		}
	}
	if t, err := time.ParseInLocation("20060102T150405", val, loc); err == nil {
		return t
	}
	if t, err := time.ParseInLocation("20060102", val, loc); err == nil {
		return t
	}
	return time.Time{}
}

func unescapeICSText(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\N`, "\n")
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return strings.TrimSpace(s)
}

var mailtoRegex = regexp.MustCompile(`(?i)mailto:([^\s;]+)`)

func parseICSOrganizer(params map[string]string, val string) string {
	cn := params["CN"]
	email := strings.TrimSpace(val)
	if m := mailtoRegex.FindStringSubmatch(val); len(m) == 2 {
		email = m[1]
	}
	if cn != "" && email != "" && !strings.EqualFold(cn, email) {
		return cn + " <" + email + ">"
	}
	if cn != "" {
		return cn
	}
	return email
}
