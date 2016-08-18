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
	Name     string  `json:"name"`
	Value    int     `json:"value"`
	Children []Child `json:"children"`
	Size     string  `json:"_size"`
}

type Performance struct {
	DownloadTime time.Duration `json:"download"`
	ParseTime    time.Duration `json:"parse"`
	ProcessTime  time.Duration `json:"process"`
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

func GetChildren(url string) (Child, Performance, error) {
	if !strings.Contains(url, "://") {
		url = "http://" + url
	}
	var root Child
	var performance Performance

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add(
		"User-Agent",
		"Mozilla/5.0 (compatible; htmltree/1.0; +https://htmltree.peterbe.com)",
	)
	t0 := time.Now()
	response, err := client.Do(req)
	if err != nil {
		return root, performance, err
	}
	defer response.Body.Close()
	t1 := time.Now()
	doc, err := html.Parse(response.Body)
	if err != nil {
		panic(err)
	}
	t2 := time.Now()

	var f func(*html.Node, int, int) []Child
	f = func(n *html.Node, depth, parentsize int) []Child {
		var children []Child
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode {
				// t0 := time.Now()
				buf := new(bytes.Buffer)
				html.Render(buf, c)
				renderedHtml := buf.String()
				// t1 := time.Now()
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
					subChildren,
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
				f(c, 0, size),
				humanize.Bytes(uint64(size)),
			}
			t3 := time.Now()
			performance.DownloadTime = t1.Sub(t0) / time.Millisecond
			performance.ParseTime = t2.Sub(t1) / time.Millisecond
			performance.ProcessTime = t3.Sub(t2) / time.Millisecond
			return root, performance, nil
		}
	}

	return root, performance, errors.New("No html root")
}

type URL struct {
	URL string `json:"url"`
}

func StringInStrings(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
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
		child, performance, err := GetChildren(url.URL)
		if err != nil {
			ctx.JSON(iris.StatusBadRequest, iris.Map{
				"error": err,
			})
			ctx.SetStatusCode(iris.StatusBadRequest) // 500
		} else {
			cacheKey := []byte("recent")
			got, err := cache.Get(cacheKey)
			// var recent string
			recent := []string{}
			if err == nil {
				recent = strings.Split(string(got), "|")
			}
			if StringInStrings(recent, url.URL) == false {
				recent = append(recent, url.URL)
			}
			// only update the cache if it wasn't already in the list
			recentAsString := strings.Join(recent, "|")
			cache.Set(cacheKey, []byte(recentAsString), 60*60*24*7) // 7 days
			ctx.JSON(iris.StatusOK, iris.Map{
				"nodes":       child,
				"performance": performance,
			})
		}
	}
}

func TreeInfo(ctx *iris.Context) {
	recent := []string{}
	cacheKey := []byte("recent")
	if got, err := cache.Get(cacheKey); err == nil {
		recent = strings.Split(string(got), "|")
		// reverse so those added most recently appear first
		for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
			recent[i], recent[j] = recent[j], recent[i]
		}
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
	if debug == true {
		api.Static("/static", "./client/static", 1)
	} else {
		api.Static("/static", "./dist/static", 1)
	}
	api.Get("/", Index)
	api.Listen("0.0.0.0:" + port)

}
