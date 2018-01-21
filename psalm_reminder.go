package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-pg/pg"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type Users struct {
	Id    int64
	Name  string
	Email string
}

type Reading struct {
	Id      int64
	Chapter string
}

type ESVResponse struct {
	Canonical string   `json:"canonical"`
	Passages  []string `json:"passages"`
}

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/email", emailHandler)

	http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}

func rootHandler(w http.ResponseWriter, _ *http.Request) {
	var psalmNum string
	options, _ := pg.ParseURL(os.Getenv("DATABASE_URL"))
	db := pg.Connect(options)
	defer db.Close()
	db.Model(&Reading{}).Column("chapter").Select(&psalmNum)
	fmt.Fprintf(w, "Psalm %q\n", psalmNum)
}

func emailHandler(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("secret")
	if p == os.Getenv("SECRET_URL_PARAMETER") {
		var psalmNum string
		var esvResponse ESVResponse
		options, _ := pg.ParseURL(os.Getenv("DATABASE_URL"))
		db := pg.Connect(options)
		defer db.Close()
		db.Model(&Reading{}).Column("chapter").Select(&psalmNum)
		url := "https://api.esv.org/v3/passage/text?q=Ps" +
			psalmNum +
			"&include-verse-numbers=false" +
			"&include-footnotes=false" +
			"&include-footnote-body=false"
		client := &http.Client{}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", os.Getenv("ESV_API_KEY"))
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}

		err = json.Unmarshal(body, &esvResponse)
		if err != nil {
			log.Fatal(err)
		}

		sendEmail(esvResponse.Passages[0], db, psalmNum)
		fmt.Fprintf(w, "Psalm %q\n", psalmNum)
	} else {
		fmt.Fprintf(w, "Psalm %q\n", "Nope")
	}
}

func sendEmail(passage string, db *pg.DB, psalmNum string) {
	from := mail.NewEmail("Reminder", "turnertgraham@gmail.com")
	subject := "Reminder"
	var user1 Users
	var user2 Users
	err := db.Model(&user1).Column("users.*").Where("users.name = ?", "Graham Turner").Select()
	if err != nil {
		panic(err)
	}
	err = db.Model(&user2).Column("users.*").Where("users.name = ?", "Alex Lowe").Select()
	if err != nil {
		panic(err)
	}
	grahamTo := mail.NewEmail("Graham Turner", user1.Email)
	alexTo := mail.NewEmail("Alex Lowe", user2.Email)
	content := mail.NewContent("text/plain", passage)
	message := mail.NewV3MailInit(from, subject, grahamTo, content)
	message.Personalizations[0].AddTos(alexTo)
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	_, err = client.Send(message)
	if err != nil {
		log.Fatal(err)
	}
	newPsalm, err := strconv.Atoi(psalmNum)
	if err == nil {
		log.Fatal(err)
	}
	newPsalm = newPsalm + 1
	newReading := &Reading{
		Chapter: string(newPsalm),
	}
	err = db.Insert(newReading)
	if err != nil {
		panic(err)
	}
}
