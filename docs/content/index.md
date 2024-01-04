---
Title: grafē
Summary: A Simple Go Static Site Generator
Tags:
- Home
- grafe
date: 24-01-04
Template: page
---

grafē is a simple static site generator written by [[https://elliberes.me|Elli Beres]].
It uses:
- [yuin](https://github.com/yuin)'s [goldmark](https://github.com/yuin/goldmark) parser and goldmark extensions for markdown parsing
- [abhinav](http://abhinavg.net/)'s [wikilink](https://go.abhg.dev/goldmark/wikilink) and [anchor](https://go.abhg.dev/goldmark/anchor) goldmark extension for wiki-style links and anchor links
- litao91's [goldmark-mathjax](https://github.com/litao91/goldmark-mathjax) goldmark extension for MathJax support

## Installation

```text
git clone https://github.com/ellifteria/grafe.git
cd grafe
go install
```

## Usage

To run grafē, run the `grafe` command in the root directory of the site.

grafē renders HTML files from Markdown files in the `./content` directory into the `./public` directory.

grafē finds HTML templates in the `./templates` and `./theme/templates` directories.
Within the template folder, grafē pages are rendered from layouts in the `templates/layouts` directory; each layout to be used in rendering includes all templates in the `templates/includes` directory.

grafē copies the contents of the `./theme/static` and the `./static` directories in that order over to `./public/static` directory.

grafē also generates a `.nojekyll` file in the `./public directory`.

A typical project structure before rendering is:

```text
.
|--+content
|  |---index.md
|--+static
|  |--+styles
|     |---newStylesheet.css
|--+templates
|  |---layouts
|  |---includes
|--+theme
   |--+static
   |  |--+styles
   |     |---stylesheet.css
   |--+templates
      |---layouts
      |---includes
```

After running grafē, this becomes:

```text
.
|--+content
|  |---index.md
|--+static
|  |--+styles
|     |---newStylesheet.css
|--+templates
|  |---layouts
|  |---includes
|--+theme
|  |--+static
|  |  |--+styles
|  |     |---stylesheet.css
|  |--+templates
|     |---layouts
|     |---includes
|--+public
   |--+static
   |  |--+styles
   |     |---newStylesheet.css
   |     |---stylesheet.css
   |---index.html
   |---.nojekyll
```
