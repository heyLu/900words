package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var settings struct {
	DailyTarget int
	Database    string
	Address     string
}

func init() {
	flag.IntVar(&settings.DailyTarget, "target", 900, "The number of words to write daily")
	flag.StringVar(&settings.Database, "db", "diary.db", "The name of the database file")
	flag.StringVar(&settings.Address, "addr", "localhost:12345", "The address of the server")
}

func main() {
	flag.Parse()

	db, err := sql.Open("sqlite3", settings.Database)
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS entries (date TEXT PRIMARY KEY, text TEXT, words INTEGER)")
	if err != nil {
		panic(err)
	}

	if flag.NArg() >= 1 {
		cmd := flag.Arg(0)
		if cmd == "import" {
			importEntry(db)
			return
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		renderEntry(w, req, db, time.Now())
	})

	http.HandleFunc("/day/", func(w http.ResponseWriter, req *http.Request) {
		parts := strings.SplitN(req.URL.Path, "/", 3)
		if len(parts) != 3 {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(http.StatusText(http.StatusNotFound)))
			return
		}

		date, err := time.Parse("2006-01-02", parts[2])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "No such day: %q\n", parts[2])
			fmt.Fprintf(os.Stderr, "Invalid date: %q: %s\n", date, err)
			return
		}

		if date.After(time.Now()) {
			w.Header().Set("Location", "/")
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}

		renderEntry(w, req, db, date)
	})

	http.HandleFunc("/save", func(w http.ResponseWriter, req *http.Request) {
		var entry map[string]string
		dec := json.NewDecoder(req.Body)
		err := dec.Decode(&entry)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err)
			return
		}

		text, ok := entry["text"]
		if !ok {
			respondWithError(w, http.StatusBadRequest, fmt.Errorf("Missing field 'text'"))
			return
		}

		now := time.Now()
		err = saveEntry(db, now, text)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, fmt.Errorf("%s", http.StatusText(http.StatusInternalServerError)))
			return
		}

		enc := json.NewEncoder(w)
		err = enc.Encode(map[string]string{"message": "saved post", "time": now.Format(time.RFC3339)})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
	})

	fmt.Printf("Starting server on http://%s\n", settings.Address)
	err = http.ListenAndServe(settings.Address, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func respondWithError(w http.ResponseWriter, status int, err error) {
	res := map[string]string{"error": err.Error()}
	out, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "{\"error\": %q}", http.StatusText(http.StatusInternalServerError))
		return
	}
	w.WriteHeader(status)
	w.Write(out)
}

func renderEntry(w http.ResponseWriter, req *http.Request, db *sql.DB, date time.Time) {
	now := time.Now()
	isSameDay := date.Format("2006-01-02") == now.Format("2006-01-02")

	if isSameDay && req.URL.Path != "/" {
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	days := daysOfMonth(date)
	annotatedDays, err := annotateDays(db, days)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}

	text := ""
	words := 0
	rows, err := db.Query("SELECT text, words FROM entries WHERE date = ?", date.Format("2006-01-02"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
	defer rows.Close()

	if rows.Next() {
		err = rows.Scan(&text, &words)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
	}

	err = indexTmpl.Execute(w, map[string]interface{}{
		"Title": fmt.Sprintf("%d words", settings.DailyTarget),

		"Now":  now,
		"Day":  date,
		"Days": annotatedDays,

		"Text":  text,
		"Words": words,

		"Editable": isSameDay,

		"Settings": settings,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	}
}

func saveEntry(db *sql.DB, date time.Time, text string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO entries VALUES (?, ?, ?)", date.Format("2006-01-02"), text, countWords(text))
	return err
}

func importEntry(db *sql.DB) {
	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s import <date>\n", os.Args[0])
		os.Exit(1)
	}

	rawDate := flag.Arg(1)
	date, err := time.Parse("2006-01-02", rawDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid date '%s': %s\n", rawDate, err)
		os.Exit(1)
	}

	f := os.Stdin
	if flag.NArg() >= 3 {
		f, err = os.Open(flag.Arg(2))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	}
	defer f.Close()

	text, err := ioutil.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: reading %s: %s\n", f.Name(), err)
		os.Exit(1)
	}

	err = saveEntry(db, date, string(text))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: saving entry: %s\n", err)
		os.Exit(1)
	}
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

		#title a {
			text-decoration: none;
			color: #000;
		}

		.month {
			margin: 0 0.5em;
		}

		#days {
			list-style-type: none;
			padding: 0;
			display: flex;
			width: 80vw;
			justify-content: space-around;
		}

		#days a {
			text-decoration: none;
			color: #000;
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
		}

		#days .yay {
			background-color: rgba(0, 255, 0, 0.5);
		}

		#days .past {
			border-color: lightgreen;
		}

		#days .future {
			color: #999;
			border-color: #ddd;
		}

		#editor {
			display: flex;
			flex-direction: column;
		}

		#editor textarea {
			width: 40em;
			height: 80vh;
			font-size: 15pt;
			font-family: serif;
			line-height: 1.6em;
			border: none;
			resize: none;
			overflow-y: hidden;
			margin-bottom: 2em;
		}

		#editor textarea:disabled {
			color: #000;
			background-color: #fff;
		}

		#editor .error {
			color: red;
		}

		#editor .success {
			color: green;
		}

		#stats {
			align-self: flex-end;
		}

		#word-count.yay {
			color: green;
			font-weight: bold;
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
			<h1 id="title"><a href="/">{{ .Title }}</a></h1>

			<h2 class="month">{{ .Day.Format "January 2006" }}</h2>

			<a href="/day/{{ ((index .Days 0).Date.AddDate 0 -1 0).Format "2006-01-02" }}">⮜</a>
			<ul id="days">
			{{ $now := .Now }}
			{{ range $day := .Days -}}
				{{ if (and ($day.Date.Before $now) (gt $day.Words 0)) }}
				<a href="/day/{{ $day.Date.Format "2006-01-02" }}" title="{{ $day.Date.Format "Mon, 02 Jan 2006" }} | {{ .Words }} words"><li class={{ $day.Classes $now }} style="background-color: rgba(0, 255, 0, {{ $day.Score }})"> {{ $day.Date.Day }}</li></a>
				{{- else }}
				<li class={{ $day.Classes $now }}>{{ $day.Date.Day }}</li>
				{{- end }}
			{{ end }}
			</ul>
			{{ if ((index .Days 0).Date.AddDate 0 +1 0).Before .Now -}}
			<a href="/day/{{ ((index .Days 0).Date.AddDate 0 +1 0).Format "2006-01-02" }}">⮞</a>
			{{ end }}

			<section id="editor">
				<h2 id="date">{{ .Day.Format "Monday, January 2, 2006" }}</h2>
				{{ if not .Editable }}<p>This day is over, so you can't change what you wrote anymore.  Try again <a href="/">today</a>.</p>{{ end }}
				<textarea id="editor" {{ if not .Editable }}disabled{{ end }}>{{ .Text }}</textarea>
				<div id="stats">
					<span id="word-count">0 words</span>
					<span id="save-status"></span>
				</div>
			</section>

			<footer>
				Made with &lt;3 by <em>strange adventures</em>.  —  <a href="/about">/about</a>
			</footer>
		</div>

		<script>
			// Settings
			var settings = {
				dailyTarget: {{ .Settings.DailyTarget }}
			};
		</script>
		<script>
			var editorEl = document.querySelector("#editor textarea");
			var wordCountEl = document.querySelector("#word-count");

			var prevCount = 0;
			function updateCount() {
				var words = editorEl.value.split(/\s+/);
				var count = words.filter(function(w) { return w.trim() != "" }).length;
				if (count != prevCount) {
					var suffix = " words";
					if (count == 1) {
						suffix = " word";
					}
					wordCountEl.textContent = count + suffix;

					if (count >= settings.dailyTarget) {
						wordCountEl.classList.add("yay");
					} else {
						wordCountEl.classList.remove("yay");
					}

					prevCount = count;
				}
			}

			function updateHeight() {
				editorEl.style.height = editorEl.scrollHeight + "px";
			}

			editorEl.addEventListener("input", function(ev) {
				updateCount();
				updateHeight();
			});

			document.addEventListener("DOMContentLoaded", function() {
				updateCount();
				updateHeight();
			});

			var statusEl = document.querySelector("#save-status");
			document.addEventListener("keydown", function(ev) {
				if (ev.ctrlKey && ev.key == 's') {
					ev.preventDefault();

					saveWords(editorEl.value);
				}
			});

			function saveWords(words) {
				statusEl.textContent = "…";

				var xhr = new XMLHttpRequest()
				xhr.open("POST", "/save")
				xhr.responseType = "json";

				function saveError() {
					statusEl.textContent = "✗";
					statusEl.classList.add("error");

					if (xhr.status == 0) {
						statusEl.title = "Could not contact server";
					} else {
						statusEl.title = xhr.response.error || "unknown error";
					}
				};

				function saveSuccess() {
					statusEl.textContent = "✓";
					statusEl.classList.remove("error");
					statusEl.title = "";
				}

				xhr.onerror = saveError;

				xhr.onload = function() {
					if (xhr.status >= 400) {
						saveError();
						return
					}

					saveSuccess();
				};

				xhr.send(JSON.stringify({text: words}));
			}
		</script>
	</body>
</html>
`))

var wordSeperator = regexp.MustCompile(`\s+`)

func countWords(text string) int {
	ws := wordSeperator.Split(text, -1)
	c := 0
	for _, w := range ws {
		if w != "" {
			c += 1
		}
	}
	return c
}

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

type Day struct {
	Date  time.Time
	Words int
}

func (d Day) Score() float32 {
	s := float32(d.Words) / float32(2*settings.DailyTarget)
	if s > 0.5 {
		return 0.5
	}
	return s
}

func (d Day) Classes(now time.Time) string {
	var classes string
	if d.Date.Before(now) {
		classes = "past"
	} else {
		classes = "future"
	}

	if d.Words >= 1 {
		classes += " written"
	}

	if d.Words >= settings.DailyTarget {
		classes += " yay"
	}

	return classes
}

func annotateDays(db *sql.DB, days []time.Time) ([]Day, error) {
	rows, err := db.Query("SELECT date, words FROM entries WHERE date >= ? AND date <= ?", days[0].Format("2006-01-02"), days[len(days)-1].Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var day string
	var words int
	if rows.Next() {
		err = rows.Scan(&day, &words)
		if err != nil {
			return nil, err
		}
	}

	annotatedDays := make([]Day, len(days))
	for i, t := range days {
		d := Day{Date: t, Words: 0}

		if t.Format("2006-01-02") == day {
			d.Words = words

			if rows.Next() {
				err = rows.Scan(&day, &words)
				if err != nil {
					return nil, err
				}
			}
		}

		annotatedDays[i] = d
	}

	return annotatedDays, nil
}
