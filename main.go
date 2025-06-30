package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

// Opravená štruktúra podľa JSON

type Media struct {
	FileName  string `json:"FileName"`
	MediaName string `json:"MediaName"`
	Duration  string `json:"Duration"`
}

type SubEvent struct {
	Duration      string     `json:"Duration"`
	EventName     string     `json:"EventName"`
	ScheduledTime string     `json:"ScheduledTime"`
	StartTime     string     `json:"StartTime"`
	Media         Media      `json:"Media"`
	SubEvents     []SubEvent `json:"SubEvents"`
}

type Rundown struct {
	Duration      string     `json:"Duration"`
	EventName     string     `json:"EventName"`
	ScheduledTime string     `json:"ScheduledTime"`
	StartTime     string     `json:"StartTime"`
	SubEvents     []SubEvent `json:"SubEvents"`
}

// XML štruktúry

type Vysilani struct {
	XMLName    xml.Name `xml:"vysilani"`
	Sirokouhle string   `xml:"sirokouhle"`
	Stereo     string   `xml:"stereo"`
}

type Porad struct {
	XMLName     xml.Name `xml:"porad"`
	ID          string   `xml:"id,attr"`
	CasOd       string   `xml:"cas-od"`
	CasDo       string   `xml:"cas-do"`
	Nazev       string   `xml:"nazev"`
	KratkyPopis string   `xml:"kratkypopis"`
	DlouhyPopis string   `xml:"dlouhypopis"`
	Vysilani    Vysilani `xml:"vysilani"`
}

type PoradDen struct {
	XMLName xml.Name `xml:"porad"`
	Datum   string   `xml:"datum,attr"`
	Porady  []Porad  `xml:"porad"`
}

type Program struct {
	XMLName  xml.Name   `xml:"program"`
	Televize string     `xml:"televize,attr"`
	Dni      []PoradDen `xml:"porad"`
}

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Program Program  `xml:"program"`
}

func rundownToXML(r io.Reader) ([]byte, error) {
	var rundown Rundown
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &rundown)
	if err != nil {
		return nil, err
	}

	datum := ""
	if len(rundown.ScheduledTime) >= 10 {
		datum = rundown.ScheduledTime[:10]
	} else if len(rundown.StartTime) >= 10 {
		datum = rundown.StartTime[:10]
	} else {
		datum = time.Now().Format("2006-01-02")
	}

	var porady []Porad
	for _, item := range rundown.SubEvents {
		casOd := ""
		if len(item.ScheduledTime) >= 16 {
			casOd = item.ScheduledTime[11:16]
			casOd = strings.ReplaceAll(casOd, ":", ".")
		}
		casDo := ""
		if len(item.StartTime) >= 16 {
			casDo = item.StartTime[11:16]
			casDo = strings.ReplaceAll(casDo, ":", ".")
		}
		nazev := item.EventName
		if item.Media.MediaName != "" {
			nazev = item.Media.MediaName
		}
		nazev = strings.ReplaceAll(nazev, "_", " ")
		if strings.Contains(strings.ToLower(nazev), "jingel") {
			continue
		}
		id := datum
		if len(item.ScheduledTime) >= 19 {
			id = strings.ReplaceAll(item.ScheduledTime[:10], "-", "")
			id += item.ScheduledTime[11:13] + item.ScheduledTime[14:16] + item.ScheduledTime[17:19]
		} else {
			id = strings.ReplaceAll(datum, "-", "") + "000000"
		}
		porady = append(porady, Porad{
			ID:          id,
			CasOd:       casOd,
			CasDo:       casDo,
			Nazev:       nazev,
			KratkyPopis: "",
			DlouhyPopis: "",
			Vysilani:    Vysilani{Sirokouhle: "Ano", Stereo: "Ano"},
		})
	}

	rss := RSS{
		Program: Program{
			Televize: "Širava",
			Dni: []PoradDen{{
				Datum:  datum,
				Porady: porady,
			}},
		},
	}

	out, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Použi POST", http.StatusMethodNotAllowed)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Chyba pri nahrávaní súboru", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reset := r.FormValue("reset") == "1"
	// Najprv vygeneruj nový deň z uploadnutého rundownu
	var rundown Rundown
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Chyba pri čítaní súboru", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(data, &rundown)
	if err != nil {
		http.Error(w, "Chyba pri parsovaní JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	datum := ""
	if len(rundown.ScheduledTime) >= 10 {
		datum = rundown.ScheduledTime[:10]
	} else if len(rundown.StartTime) >= 10 {
		datum = rundown.StartTime[:10]
	} else {
		datum = time.Now().Format("2006-01-02")
	}

	var porady []Porad
	for _, item := range rundown.SubEvents {
		casOd := ""
		if len(item.ScheduledTime) >= 16 {
			casOd = item.ScheduledTime[11:16]
			casOd = strings.ReplaceAll(casOd, ":", ".")
		}
		casDo := ""
		if len(item.StartTime) >= 16 {
			casDo = item.StartTime[11:16]
			casDo = strings.ReplaceAll(casDo, ":", ".")
		}
		nazev := item.EventName
		if item.Media.MediaName != "" {
			nazev = item.Media.MediaName
		}
		nazev = strings.ReplaceAll(nazev, "_", " ")
		if strings.Contains(strings.ToLower(nazev), "jingel") {
			continue
		}
		id := datum
		if len(item.ScheduledTime) >= 19 {
			id = strings.ReplaceAll(item.ScheduledTime[:10], "-", "")
			id += item.ScheduledTime[11:13] + item.ScheduledTime[14:16] + item.ScheduledTime[17:19]
		} else {
			id = strings.ReplaceAll(datum, "-", "") + "000000"
		}
		porady = append(porady, Porad{
			ID:          id,
			CasOd:       casOd,
			CasDo:       casDo,
			Nazev:       nazev,
			KratkyPopis: "",
			DlouhyPopis: "",
			Vysilani:    Vysilani{Sirokouhle: "Ano", Stereo: "Ano"},
		})
	}

	xmlPath := "output.xml"
	var rss RSS
	if !reset {
		if _, err := os.Stat(xmlPath); err == nil {
			// Súbor existuje, načítaj
			xmlData, err := ioutil.ReadFile(xmlPath)
			if err == nil {
				xml.Unmarshal(xmlData, &rss)
			}
		}
	}

	// Pridaj nový deň alebo porady k existujúcemu dňu
	dayExist := false
	for i, den := range rss.Program.Dni {
		if den.Datum == datum {
			rss.Program.Dni[i].Porady = append(rss.Program.Dni[i].Porady, porady...)
			dayExist = true
			break
		}
	}
	if !dayExist {
		rss.Program.Dni = append(rss.Program.Dni, PoradDen{
			Datum:  datum,
			Porady: porady,
		})
	}
	if rss.Program.Televize == "" {
		rss.Program.Televize = "Širava"
	}

	out, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		http.Error(w, "Chyba pri generovaní XML: "+err.Error(), http.StatusInternalServerError)
		return
	}
	err = ioutil.WriteFile(xmlPath, append([]byte(xml.Header), out...), 0644)
	if err != nil {
		http.Error(w, "Chyba pri zápise XML: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", "attachment; filename=sirava.xml") // TODO
	w.Write(append([]byte(xml.Header), out...))
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		http.HandleFunc("/upload", uploadHandler)
		http.Handle("/", http.FileServer(http.Dir(".")))
		fmt.Println("Server beží na http://localhost:8080 ...")
		http.ListenAndServe(":8080", nil)
		return
	}

	// CLI režim (pôvodné správanie)
	jsonFile, err := os.Open("./sirava.rundown")
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer jsonFile.Close()
	xmlData, err := rundownToXML(jsonFile)
	if err != nil {
		fmt.Println("Chyba pri generovaní XML:", err)
		return
	}
	f, err := os.Create("output.xml")
	if err != nil {
		fmt.Println("Error creating XML file:", err)
		return
	}
	defer f.Close()
	f.Write(xmlData)
	fmt.Println("Vygenerované output.xml")
}
