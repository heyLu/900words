package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		now := time.Now()
		err := indexTmpl.Execute(w, map[string]interface{}{
			"Title": "900 words",

			"Now":  now,
			"Days": daysOfMonth(now),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
	})
	http.ListenAndServe("localhost:12345", nil)
}

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<html>
	<head>
		<meta charset="utf-8" />
		<title>{{ .Title }}</title>

		<style>
		#content {
			display: flex;
			flex-direction: column;
			align-items: center;
		}

		#days {
			list-style-type: none;
			margin-bottom: 2em;
			padding: 0;
			display: flex;
			width: 80vw;
			justify-content: space-around;
		}

		#days li {
			width: 1.5em;
			height: 1.5em;
			text-align: center;
			border: 1px solid;
			border-radius: 100%;
		}

		#days .written {
			background-color: rgba(0, 255, 0, 0.2);
			background-color: green;
		}

		#days .past {
			border-color: lightgreen;
		}

		#days .future {
			color: #999;
			border-color: #ddd;
		}

		#editor textarea {
			width: 40em;
			height: 80vh;
			font-size: 15pt;
			font-family: serif;
			border: none;
			resize: none;
		}

		footer {
			color: #999;
		}

		footer a, footer a:visited {
			color: #999;
		}
		</style>
	</head>

	<body>
		<div id="content">
			<h1>{{ .Title }}</h1>

			<ul id="days">
			{{ $now := .Now }}
			{{ range $day := .Days -}}
			<li{{ if ($day.After $now) }} class="future"{{ else }} class="past"{{ end }}>{{ $day.Day }}</li> 
			{{ end }}
			</ul>

			<section id="editor">
				<textarea id="editor"></textarea>
			</section>

			<footer>
				Made with &lt;3 by strange adventures.  <a href="/about">/about</a>
			</footer>
		</div>
	</body>
</html>
`))

func daysOfMonth(t time.Time) []time.Time {
	s := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	e := s.AddDate(0, 1, 0)
	ts := make([]time.Time, 0, 31)
	for s.Before(e) {
		ts = append(ts, s)
		s = s.AddDate(0, 0, 1)
	}
	return ts
}
