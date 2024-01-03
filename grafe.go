package main

import (
	"bytes"
	"html/template"
	"io"
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

	"go.abhg.dev/goldmark/wikilink"

	mathjax "github.com/litao91/goldmark-mathjax"
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

func createFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		return nil, err
	}
	return os.Create(path)
}

func copyFile(sourcePath string, destinationPath string) {
	source, err := os.Open(sourcePath)
	check(err)
	defer source.Close()

	destination, err := os.Create(destinationPath)
	check(err)

	defer destination.Close()
	_, err = io.Copy(destination, source)
	check(err)
}

func isMdFile(filePath string) bool {
	extension := func(name string) string {
		elems := strings.Split(strings.TrimSpace(name), ".")
		return elems[len(elems)-1]
	}
	return extension(filePath) == "md"
}

func parseTemplateDirectory(directory string) (*template.Template, error) {
	var paths []string

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		check(err)

		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})

	check(err)

	return template.ParseFiles(paths...)
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

	htmlTemplate, err := parseTemplateDirectory("templates")
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

	templateType := metaData["Template"].(string) + ".html"

	err = htmlTemplate.ExecuteTemplate(file, templateType, data)
	check(err)
}

func main() {
	markdownWriter := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
			extension.Table,
			&wikilink.Extender{},
			mathjax.MathJax,
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(
					extension.NewTableHTMLRenderer(),
					500,
				),
			),
			html.WithUnsafe(),
		),
	)

	err := os.RemoveAll("public")
	check(err)

	walk("content", func(fileName string) {
		if isMdFile(fileName) {
			fileData, err := os.ReadFile(fileName)
			check(err)
			generateHtmlFile(
				markdownWriter,
				string(fileData),
				"public/"+strings.TrimPrefix(
					changeExtension(fileName, ".html"),
					"content/",
				),
			)
		} else {
			newFileName := strings.TrimPrefix(fileName, "content/")
			createFile("public/" + newFileName)
			copyFile(
				fileName,
				"public/"+newFileName,
			)
		}
	})

	walk("static", func(fileName string) {
		createFile("public/" + fileName)
		copyFile(
			fileName,
			"public/"+fileName,
		)
	})

	_, err = os.Create("public/.nojekyll")
	check(err)
}
