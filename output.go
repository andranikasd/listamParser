package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"time"
)

// loadSeen reads an existing data.json and returns the set of known hashes.
func loadSeen(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(raw))
	for k := range raw {
		seen[k] = true
	}
	return seen, nil
}

// saveJSON writes the current announcements as the new state file.
func saveJSON(path string, anns []*Announcement) error {
	type entry struct {
		SqPrice float64 `json:"sq_price"`
		Title   string  `json:"title"`
		Price   int     `json:"price"`
		Link    string  `json:"link"`
	}
	m := make(map[string]entry, len(anns))
	for _, a := range anns {
		m[a.Hash()] = entry{a.SqPrice, a.Title, a.Price, a.Link}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// writeCSV writes all announcements to a CSV file.
func writeCSV(path string, anns []*Announcement) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"Link", "Title", "Price ($)", "Image URL", "Region", "Labels", "Floor", "Max Floor", "Size (m²)", "Rooms", "$/m²"})
	for _, a := range anns {
		_ = w.Write([]string{
			a.Link, a.Title,
			strconv.Itoa(a.Price),
			a.ImageURL, a.Region, a.Labels,
			a.ActualFloor, a.MaxFloor,
			strconv.Itoa(a.Size),
			strconv.Itoa(a.Rooms),
			fmt.Sprintf("%.2f", a.SqPrice),
		})
	}
	return w.Error()
}

// writeHTML generates a standalone searchable/sortable HTML table.
func writeHTML(path string, anns []*Announcement, mode string) error {
	tmpl, err := template.New("t").Parse(htmlTmpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, struct {
		Mode, GeneratedAt string
		Announcements     []*Announcement
	}{mode, time.Now().Format("2006-01-02 15:04"), anns})
}

const htmlTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>list.am – {{.Mode}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#eef1f7;color:#1a1a2e}
header{background:#1565c0;color:#fff;padding:.85rem 1.5rem;display:flex;align-items:center;justify-content:space-between;gap:.5rem}
header h1{font-size:1.05rem;font-weight:700;letter-spacing:-.01em}
header .ts{font-size:.78rem;opacity:.75}
.bar{padding:.65rem 1.5rem;display:flex;align-items:center;gap:.75rem;flex-wrap:wrap;background:#fff;border-bottom:1px solid #dde3ec}
#q{flex:1;min-width:200px;max-width:380px;padding:.42rem .75rem;border:1.5px solid #b0bec5;border-radius:6px;font-size:.88rem}
#q:focus{outline:none;border-color:#1565c0;box-shadow:0 0 0 3px rgba(21,101,192,.18)}
#info{font-size:.8rem;color:#607d8b;white-space:nowrap}
.wrap{overflow-x:auto;margin:1rem 1.25rem 2.5rem;border-radius:8px;box-shadow:0 2px 10px rgba(0,0,0,.1)}
table{border-collapse:collapse;width:100%;background:#fff}
thead tr{background:#1565c0;color:#fff}
th{padding:.52rem .85rem;font-size:.79rem;font-weight:600;text-align:left;white-space:nowrap;cursor:pointer;user-select:none;transition:background .15s}
th:hover{background:#0d47a1}
th.asc::after{content:" ↑";font-size:.68rem;opacity:.9}
th.desc::after{content:" ↓";font-size:.68rem;opacity:.9}
td{padding:.48rem .85rem;font-size:.8rem;border-bottom:1px solid #eceff1;vertical-align:middle}
tbody tr:last-child td{border-bottom:0}
tbody tr:hover td{background:#e3f2fd}
a{color:#1565c0;text-decoration:none}
a:hover{text-decoration:underline}
.badge{display:inline-block;background:#e3f2fd;color:#1565c0;border-radius:4px;padding:2px 8px;font-size:.74rem;white-space:nowrap}
.sp{font-weight:700;color:#2e7d32}
.hidden{display:none!important}
@media(max-width:600px){.wrap{margin:.5rem .25rem}}
</style>
</head>
<body>
<header>
  <h1>list.am &mdash; {{.Mode}} listings</h1>
  <span class="ts">generated {{.GeneratedAt}}</span>
</header>
<div class="bar">
  <input id="q" type="search" placeholder="Filter by title, region, label…" oninput="filt()">
  <span id="info"></span>
</div>
<div class="wrap">
<table id="T">
<thead><tr>
  <th onclick="srt(0)">$/m²</th>
  <th onclick="srt(1)">Price ($)</th>
  <th onclick="srt(2)">Size m²</th>
  <th onclick="srt(3)">Rooms</th>
  <th onclick="srt(4)">Region</th>
  <th onclick="srt(5)">Floor</th>
  <th onclick="srt(6)">Title</th>
  <th>Labels</th>
</tr></thead>
<tbody>
{{- range .Announcements}}
<tr>
  <td class="sp" data-v="{{printf "%.4f" .SqPrice}}">{{printf "%.0f" .SqPrice}}</td>
  <td data-v="{{.Price}}">{{.Price}}</td>
  <td data-v="{{.Size}}">{{.Size}}</td>
  <td data-v="{{.Rooms}}">{{.Rooms}}</td>
  <td>{{.Region}}</td>
  <td>{{.ActualFloor}}/{{.MaxFloor}}</td>
  <td><a href="{{.Link}}" target="_blank" rel="noopener noreferrer">{{.Title}}</a></td>
  <td>{{if .Labels}}<span class="badge">{{.Labels}}</span>{{end}}</td>
</tr>
{{- end}}
</tbody>
</table>
</div>
<script>
let sc=-1,asc=true;
const T=document.getElementById('T');
function upd(){
  const rows=[...T.tBodies[0].rows];
  const v=rows.filter(r=>!r.classList.contains('hidden')).length;
  document.getElementById('info').textContent=v+' / '+rows.length+' listings';
}
function filt(){
  const q=document.getElementById('q').value.toLowerCase();
  for(const r of T.tBodies[0].rows)
    r.classList.toggle('hidden',!!q&&!r.textContent.toLowerCase().includes(q));
  upd();
}
function srt(c){
  if(sc===c){asc=!asc}else{sc=c;asc=true}
  for(const th of T.tHead.rows[0].cells){
    th.classList.remove('asc','desc');
    if(th.cellIndex===c)th.classList.add(asc?'asc':'desc');
  }
  const rows=[...T.tBodies[0].rows];
  rows.sort((a,b)=>{
    const av=a.cells[c].dataset.v??a.cells[c].textContent.trim();
    const bv=b.cells[c].dataset.v??b.cells[c].textContent.trim();
    const na=+av,nb=+bv;
    const d=isNaN(na)||isNaN(nb)?av.localeCompare(bv,'en',{sensitivity:'base'}):na-nb;
    return asc?d:-d;
  });
  rows.forEach(r=>T.tBodies[0].appendChild(r));
  upd();
}
upd();srt(0);
</script>
</body>
</html>`
