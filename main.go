package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/coocood/freecache"
	"github.com/dustin/go-humanize"
	"github.com/kataras/iris"
	"github.com/kataras/iris/config"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Child struct {
	Name      string        `json:"name"`
	Value     int           `json:"value"`
	Ratio     float64       `json:"percentage"`
	Children  []Child       `json:"children"`
	FromCache bool          `json:"_from_cache"`
	Took      time.Duration `json:"_took"`
	Size      string        `json:"_size"`
}

func DescribeNode(n *html.Node, size int) string {
	attrs := ""
	for _, attr := range n.Attr {
		if attr.Key == "class" || attr.Key == "id" {
			attrs += fmt.Sprintf(" %s=\"%s\"", attr.Key, attr.Val)
		}
	}
	return fmt.Sprintf("<%s%s> %s", n.Data, attrs, humanize.Bytes(uint64(size)))
}

func GetChildren(url string) (Child, error) {
	if !strings.Contains(url, "://") {
		url = "http://" + url
	}
	var root Child

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add(
		"User-Agent",
		"Mozilla/5.0 (compatible; htmltree/1.0; +https://htmltree.peterbe.com)",
	)
	t0 := time.Now()
	response, err := client.Do(req)
	if err != nil {
		return root, err
	}
	defer response.Body.Close()
	// t1 := time.Now()
	doc, err := html.Parse(response.Body)
	if err != nil {
		panic(err)
	}
	// t1 := time.Now()
	t2 := time.Now()
	// elapsed := time.Since(t0)

	var f func(*html.Node, int, int) []Child
	f = func(n *html.Node, depth, parentsize int) []Child {
		var children []Child
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				t0 := time.Now()
				buf := new(bytes.Buffer)
				html.Render(buf, c)
				renderedHtml := buf.String()
				t1 := time.Now()
				size := len(renderedHtml)
				var subChildren []Child
				if depth < 5 {
					subChildren = f(c, depth+1, size)
				} else {
					subChildren = make([]Child, 0)
				}

				child := Child{
					DescribeNode(c, size),
					size,
					float64(size) / float64(parentsize),
					subChildren,
					false,
					t1.Sub(t0),
					humanize.Bytes(uint64(size)),
				}
				children = append(children, child)

			}
		}
		return children
	}

	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			buf := new(bytes.Buffer)
			html.Render(buf, c)
			renderedHtml := buf.String()
			size := len(renderedHtml)
			root := Child{
				DescribeNode(c, size),
				size,
				1.0,
				f(c, 0, size),
				false,
				t2.Sub(t0) / time.Millisecond,
				humanize.Bytes(uint64(size)),
			}
			return root, nil
		}
	}

	return root, errors.New("No html root")
}

type URL struct {
	URL string `json:"url"`
}

func Tree(ctx *iris.Context) {
	url := URL{}
	jsonErr := ctx.ReadJSON(&url)
	if jsonErr != nil {
		ctx.JSON(iris.StatusBadRequest, iris.Map{
			"error": "No 'url'",
		})
		ctx.SetStatusCode(iris.StatusBadRequest) // 400
	} else {
		child, err := GetChildren(url.URL)
		if err != nil {
			ctx.JSON(iris.StatusBadRequest, iris.Map{
				"error": err,
			})
			ctx.SetStatusCode(iris.StatusBadRequest) // 500
		} else {
			cacheKey := []byte("recent")
			got, err := cache.Get(cacheKey)
			var recent string
			if err != nil {
				// never stored before
				recent = ""
			} else {
				// prepare concatenation
				recent = fmt.Sprintf("|%s", string(got))
			}
			recent = fmt.Sprintf("%s%s", url.URL, recent)
			cache.Set(cacheKey, []byte(recent), 60*60*24*7) // 7 days
			ctx.JSON(iris.StatusOK, child)
		}
	}
}

func TreeInfo(ctx *iris.Context) {
	recent := []string{}
	cacheKey := []byte("recent")
	if got, err := cache.Get(cacheKey); err == nil {
		recent = strings.Split(string(got), "|")
	}
	ctx.JSON(iris.StatusOK, iris.Map{
		"jobs":   0,
		"recent": recent,
	})
}

var (
	debug     bool
	cache     *freecache.Cache
	cacheSize = 1024 * 1024 // 1Mb
)

func Index(ctx *iris.Context) {
	if debug == true {
		ctx.ServeFile("./client/index.html", false)
	} else {
		ctx.ServeFile("./dist/index.html", false)
	}
}

func main() {
	cache = freecache.NewCache(cacheSize)
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}
	debug = os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1"
	irisConfig := config.Iris{
		IsDevelopment: debug,
	}
	api := iris.New(irisConfig)
	api.Get("/tree", TreeInfo)
	api.Post("/tree", Tree)
	api.Static("/static", "./dist/static", 1)
	api.Get("/", Index)
	api.Listen("0.0.0.0:" + port)

}
