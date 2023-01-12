package grabber

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"

	"github.com/elboletaire/manga-downloader/http"
)

// Mangadex is a grabber for mangadex.org
type Mangadex struct {
	Grabber
	title string
}

// MangadexChapter represents a MangaDex Chapter
type MangadexChapter struct {
	Chapter
	Id string
}

// Test checks if the site is MangaDex
func (m *Mangadex) Test() bool {
	re := regexp.MustCompile(`mangadex\.org`)
	return re.MatchString(m.URL)
}

// GetTitle returns the title of the manga
func (m *Mangadex) GetTitle() string {
	if m.title != "" {
		return m.title
	}

	id := GetUUID(m.URL)

	rbody, err := http.Get(http.RequestParams{
		URL:     "https://api.mangadex.org/manga/" + id,
		Referer: m.GetBaseUrl(),
	})
	if err != nil {
		panic(err)
	}
	defer rbody.Close()
	body := MangadexManga{}
	if err = json.NewDecoder(rbody).Decode(&body); err != nil {
		panic(err)
	}

	// fetch the title in the requested language
	if m.PreferredLanguage != "" {
		trans := body.Data.Attributes.AltTitles.GetTitleByLang(m.PreferredLanguage)

		if trans != "" {
			m.title = trans
			return m.title
		}
	}

	// fallback to english
	m.title = body.Data.Attributes.Title["en"]
	return m.title
}

// FetchChapters returns the chapters of the manga
func (m Mangadex) FetchChapters() Filterables {
	id := GetUUID(m.URL)

	var chapters Filterables
	var fetchChaps func(int)

	baseOffset := 500

	fetchChaps = func(offset int) {
		uri, err := url.JoinPath("https://api.mangadex.org", "manga", id, "feed")
		if err != nil {
			panic(err)
		}
		params := url.Values{}
		params.Add("limit", fmt.Sprint(baseOffset))
		params.Add("order[volume]", "asc")
		params.Add("order[chapter]", "asc")
		params.Add("offset", fmt.Sprint(offset))
		if m.PreferredLanguage != "" {
			params.Add("translatedLanguage[]", m.PreferredLanguage)
		}
		uri = fmt.Sprintf("%s?%s", uri, params.Encode())

		rbody, err := http.Get(http.RequestParams{URL: uri})
		if err != nil {
			panic(err)
		}
		defer rbody.Close()
		body := MangadexFeed{}
		err = json.NewDecoder(rbody).Decode(&body)
		if err != nil {
			panic(err)
		}

		for _, c := range body.Data {
			num, _ := strconv.ParseFloat(c.Attributes.Chapter, 64)
			chapters = append(chapters, &MangadexChapter{
				Chapter{
					Number:   num,
					Title:    c.Attributes.Title,
					Language: c.Attributes.TranslatedLanguage,
				},
				c.Id,
			})
		}

		if len(body.Data) > 0 {
			fetchChaps(offset + baseOffset)
		}
	}
	fetchChaps(0)

	return chapters
}

// FetchChapter fetches a chapter and its pages
func (m Mangadex) FetchChapter(f Filterable) Chapter {
	chap := f.(*MangadexChapter)
	// download json
	rbody, err := http.Get(http.RequestParams{
		URL: "https://api.mangadex.org/at-home/server/" + chap.Id,
	})
	if err != nil {
		panic(err)
	}
	// parse json body
	body := MangadexPagesFeed{}
	if err = json.NewDecoder(rbody).Decode(&body); err != nil {
		panic(err)
	}

	chapter := Chapter{
		Title:      fmt.Sprintf("Chapter %04d %s", int64(f.GetNumber()), chap.Title),
		Number:     f.GetNumber(),
		PagesCount: int64(len(body.Chapter.Data)),
		Language:   chap.Language,
	}

	// create pages
	for i, p := range body.Chapter.Data {
		chapter.Pages = append(chapter.Pages, Page{
			Number: int64(i + 1),
			URL:    body.BaseUrl + path.Join("/data", body.Chapter.Hash, p),
		})
	}

	return chapter
}

// MangadexManga represents the Manga json object
type MangadexManga struct {
	Id   string
	Data struct {
		Attributes struct {
			Title     map[string]string
			AltTitles AltTitles
		}
	}
}

// AltTitles is a slice of maps with the language as key and the title as value
type AltTitles []map[string]string

// GetTitleByLang returns the title in the given language (or empty if string is not found)
func (a AltTitles) GetTitleByLang(lang string) string {
	for _, t := range a {
		val, ok := t[lang]
		if ok {
			return val
		}
	}
	return ""
}

// MangadexFeed represents the json object returned by the feed endpoint
type MangadexFeed struct {
	Data []struct {
		Id         string
		Attributes struct {
			Volume             string
			Chapter            string
			Title              string
			TranslatedLanguage string
		}
	}
}

// MangadexPagesFeed represents the json object returned by the pages endpoint
type MangadexPagesFeed struct {
	BaseUrl string
	Chapter struct {
		Hash      string
		Data      []string
		DataSaver []string
	}
}
