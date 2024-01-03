package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func walk(dir string, fileFunction func(filePath string)) {
	items, _ := os.ReadDir(dir)
	for _, item := range items {
		if !item.IsDir() {
			fileFunction(dir + "/" + item.Name())
		} else {
			walk(dir+"/"+item.Name(), fileFunction)
		}
	}
}

func changeExtension(filePath string, newExtension string) string {
	extension := func(name string) string {
		elems := strings.Split(strings.TrimSpace(name), ".")
		return elems[len(elems)-1]
	}
	return strings.TrimSuffix(filePath, "."+extension(filePath)) + newExtension
}

func isMdFile(filePath string) bool {
	extension := func(name string) string {
		elems := strings.Split(strings.TrimSpace(name), ".")
		return elems[len(elems)-1]
	}
	return extension(filePath) == "md"
}

func createFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		return nil, err
	}
	return os.Create(path)
}

func generateHtmlFile(markdownWriter goldmark.Markdown, sourceMd string, outputFile string) {
	var buf bytes.Buffer
	var err error

	file, err := createFile(outputFile)
	check(err)
	defer file.Close()

	context := parser.NewContext()
	err = markdownWriter.Convert([]byte(sourceMd), &buf, parser.WithContext(context))
	check(err)
	metaData := meta.Get(context)
	fmt.Fprintln(os.Stderr, metaData)

	var templateFile = "templates/base.html"
	t, err := template.New("base.html").ParseFiles(templateFile)
	check(err)

	data := struct {
		Title   string
		Summary string
		Body    template.HTML
	}{
		Title:   metaData["Title"].(string),
		Summary: metaData["Summary"].(string),
		Body:    template.HTML(buf.String()),
	}

	err = t.Execute(file, data)
	check(err)
}

func main() {
	markdownWriter := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
			extension.Table,
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(extension.NewTableHTMLRenderer(), 500),
			),
			html.WithUnsafe(),
		),
	)

	err := os.RemoveAll("output")
	check(err)

	walk("content", func(fileName string) {
		if !isMdFile(fileName) {
			return
		}
		fileData, err := os.ReadFile(fileName)
		check(err)
		generateHtmlFile(markdownWriter, string(fileData), "output/"+changeExtension(fileName, ""))
	})
}
