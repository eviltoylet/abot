package datatypes

import (
	"database/sql/driver"
	"encoding/csv"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

type StringSlice []string

var (
	ErrInvalidClass        = errors.New("invalid class")
	ErrInvalidOddParameter = errors.New("parameter count must be even")
)

const (
	CommandI int = iota + 1
	ActorI
	ObjectI
	TimeI
	PlaceI
	NoneI
)

const (
	FlexIdTypeEmail int = iota + 1
	FlexIdTypePhone
)

var String map[int]string = map[int]string{
	CommandI: "Command",
	ActorI:   "Actor",
	ObjectI:  "Object",
	TimeI:    "Time",
	PlaceI:   "Place",
	NoneI:    "None",
}

var Pronouns map[string]int = map[string]int{
	"me":    ActorI,
	"us":    ActorI,
	"you":   ActorI,
	"him":   ActorI,
	"her":   ActorI,
	"them":  ActorI,
	"it":    ObjectI,
	"that":  ObjectI,
	"there": PlaceI,
	"then":  TimeI,
}

// StructuredInput is generated by Ava and sent to packages. The UserId is
// guaranteed to be unique. The FlexId is used for UserId lookups to maintain
// context, such as a phone number or email address.
type StructuredInput struct {
	UserId     int
	FlexId     string
	FlexIdType int
	Sentence   string
	Commands   StringSlice
	Actors     StringSlice
	Objects    StringSlice
	Times      StringSlice
	Places     StringSlice
}

type User struct {
	Id                int
	Email             string
	Phone             string
	LastAuthenticated *time.Time
}

func (si *StructuredInput) String() string {
	s := "\n"
	if len(si.Commands) > 0 {
		s += "Command: " + strings.Join(si.Commands, ", ") + "\n"
	}
	if len(si.Actors) > 0 {
		s += "Actors: " + strings.Join(si.Actors, ", ") + "\n"
	}
	if len(si.Objects) > 0 {
		s += "Objects: " + strings.Join(si.Objects, ", ") + "\n"
	}
	if len(si.Times) > 0 {
		s += "Times: " + strings.Join(si.Times, ", ") + "\n"
	}
	if len(si.Places) > 0 {
		s += "Places: " + strings.Join(si.Places, ", ") + "\n"
	}
	return s[:len(s)-1] + "\n"
}

type WordClass struct {
	Word  string
	Class int
}

// Add pairs of words with their classes to a structured input. Params should
// follow the ("Order", "Command"), ("noon", "Time") form.
func (si *StructuredInput) Add(wc []WordClass) error {
	if len(wc) == 0 {
		return ErrInvalidOddParameter
	}
	for _, w := range wc {
		switch w.Class {
		case CommandI:
			si.Commands = append(si.Commands, w.Word)
		case ActorI:
			si.Actors = append(si.Actors, w.Word)
		case ObjectI:
			si.Objects = append(si.Objects, w.Word)
		case TimeI:
			si.Times = append(si.Times, w.Word)
		case PlaceI:
			si.Places = append(si.Places, w.Word)
		case NoneI:
			// Do nothing
		default:
			log.Error("invalid class: ", w.Class)
			return ErrInvalidClass
		}
	}
	return nil
}

// TODO Optimize by passing back a struct with []string AND int (ActorI,
// ObjectI, etc.)
func (si *StructuredInput) Pronouns() []string {
	p := []string{}
	for _, w := range si.Objects {
		if Pronouns[w] != 0 {
			p = append(p, w)
		}
	}
	for _, w := range si.Actors {
		if Pronouns[w] != 0 {
			p = append(p, w)
		}
	}
	for _, w := range si.Times {
		if Pronouns[w] != 0 {
			p = append(p, w)
		}
	}
	for _, w := range si.Places {
		if Pronouns[w] != 0 {
			p = append(p, w)
		}
	}
	return p
}

// for replacing escaped quotes except if it is preceded by a literal backslash
//  eg "\\" should translate to a quoted element whose value is \

var quoteEscapeRegex = regexp.MustCompile(`([^\\]([\\]{2})*)\\"`)

// Scan convert to a slice of strings
// http://www.postgresql.org/docs/9.1/static/arrays.html#ARRAYS-IO
func (s *StringSlice) Scan(src interface{}) error {
	asBytes, ok := src.([]byte)
	if !ok {
		return error(errors.New("scan source was not []bytes"))
	}
	str := string(asBytes)
	str = quoteEscapeRegex.ReplaceAllString(str, `$1""`)
	str = strings.Replace(str, `\\`, `\`, -1)
	str = str[1 : len(str)-1]
	csvReader := csv.NewReader(strings.NewReader(str))
	slice, err := csvReader.Read()
	if err != nil && err.Error() != "EOF" {
		return err
	}
	*s = StringSlice(slice)
	return nil
}

func (s StringSlice) Value() (driver.Value, error) {
	// string escapes.
	// \ => \\\
	// " => \"
	for i, elem := range s {
		s[i] = `"` + strings.Replace(strings.Replace(elem, `\`, `\\\`, -1), `"`, `\"`, -1) + `"`
	}
	return "{" + strings.Join(s, ",") + "}", nil
}

func (s StringSlice) Last() string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

func (u *User) isAuthenticated() (bool, error) {
	var oldTime time.Time
	tmp := os.Getenv("REQUIRE_AUTH_IN_HOURS")
	var t int
	if len(tmp) > 0 {
		var err error
		t, err = strconv.Atoi(tmp)
		if err != nil {
			return false, err
		}
		if t < 0 {
			return false, errors.New("negative REQUIRE_AUTH_IN_HOURS")
		}
	} else {
		log.Warn("REQUIRE_AUTH_IN_HOURS environment variable is not set.",
			" Using 168 hours (one week) as the default.")
		t = 168
	}
	oldTime = time.Now().Add(time.Duration(-1*t) * time.Hour)
	authenticated := false
	if u.LastAuthenticated.After(oldTime) {
		authenticated = true
	}
	return authenticated, nil
}
