package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/kataras/iris"
	"github.com/kataras/iris/config"
	"golang.org/x/net/html"
	"io/ioutil"
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
	t0 := time.Now()
	response, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	doc, err := html.Parse(response.Body)
	if err != nil {
		panic(err)
	}
	// t1 := time.Now()
	elapsed := time.Since(t0)

	var root Child

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
				elapsed / time.Millisecond,
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
			ctx.JSON(iris.StatusOK, child)
		}
	}
}

func TreeInfo(ctx *iris.Context) {
	recent := []string{}
	ctx.JSON(iris.StatusOK, iris.Map{
		"jobs":   0,
		"recent": recent,
	})
}

func Index(ctx *iris.Context) {
	ctx.ServeFile("./dist/index.html", false)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}
	debug := os.Getenv("DEBUG")
	irisConfig := config.Iris{
		// DisablePathCorrection: true,
		IsDevelopment: debug == "true",
	}
	api := iris.New(irisConfig)
	api.Get("/tree", TreeInfo)
	api.Post("/tree", Tree)
	api.Static("/static", "./dist/static", 1)
	api.Get("/", Index)
	api.Listen("0.0.0.0:" + port)

}
