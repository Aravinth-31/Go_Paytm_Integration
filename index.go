package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	PaytmChecksum "./Paytm_Go_Checksum/paytm"
	"github.com/go-yaml/yaml"
	_ "github.com/lib/pq"
)

var database *sql.DB

type Config struct {
	PaymentParams struct {
		Port           string `yaml:"port"`
		MID            string `yaml:"MID"`
		WEBSITE        string `yaml:"WEBSITE"`
		CHANNELID      string `yaml:"CHANNEL_ID"`
		INDUSTRYTYPEID string `yaml:"INDUSTRY_TYPE_ID"`
		ORDERID        string `yaml:"ORDER_ID"`
		CUSTID         string `yaml:"CUST_ID"`
		CALLBACKURL    string `yaml:"CALLBACK_URL"`
		KEY            string `yaml:"KEY"`
		TXNURL         string `yaml:"TXNURL"`
		TXNSTATUSURL   string `yaml:"TXNSTATUSURL"`
	} `yaml:"paymentParams"`
	DataBase struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Dbname   string `yaml:"dbname"`
	} `yaml:"database"`
}

var config Config

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

type Result struct {
	TXNID       string
	BANKTXNID   string
	ORDERID     string
	TXNAMOUNT   string
	STATUS      string
	TXNTYPE     string
	GATEWAYNAME string
	RESPCODE    string
	RESPMSG     string
	BANKNAME    string
	MID         string
	PAYMENTMODE string
	REFUNDAMT   string
	TXNDATE     string
}

//PaymentHandler is ...
func PaymentHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `	<style>
					#form{width:400px;margin-left:40%;margin-top:100px;border:2px solid black;justify-content:center}
					input{width:80%;padding:10px;margin:20px 40px}
					input[type="submit"]{background-color:blue;color:white;width:75%;font-weight:bolder}
					h1{margin:20px}
					</style>
					<div id="form"><h1 style="color:red">Payment</h1>
					<input type="numbers" style="width:300px;padding:10px 20px" id="amt" placeholder="Enter Amount"/><br/>
					<input type="numbers" style="width:300px;padding:10px 20px" id="num" placeholder="Enter Phone Number"/><br/>
					<input type="email" style="width:300px;padding:10px 20px" id="email" placeholder="Enter Email"/><br/>
					<input type="submit" onclick="send()" value="Pay"/></div>
					<script type="text/javascript">function send(){
						var amount=document.getElementById("amt").value;
						var value=parseFloat(amount.toString())
						var number=document.getElementById("num").value;
						var email=document.getElementById("email").value;
						window.location.replace("http://localhost:3000/pay?amt="+(value.toFixed(2)).toString()+"&number="+number+"&email="+email);
						}</script>
`)
}

//IndexHandler is
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	Amount := r.FormValue("amt")
	PhnNumber := r.FormValue("number")
	Email := r.FormValue("email")
	paytmParams := make(map[string]string)
	paytmParams = map[string]string{
		"MID":              config.PaymentParams.MID,
		"WEBSITE":          config.PaymentParams.WEBSITE,
		"CHANNEL_ID":       config.PaymentParams.CHANNELID,
		"INDUSTRY_TYPE_ID": config.PaymentParams.INDUSTRYTYPEID,
		"ORDER_ID":         config.PaymentParams.ORDERID + time.Now().Format("20060102150405"),
		"CUST_ID":          config.PaymentParams.CUSTID,
		"TXN_AMOUNT":       Amount,
		"CALLBACK_URL":     config.PaymentParams.CALLBACKURL,
		"EMAIL":            Email,
		"MOBILE_NO":        PhnNumber,
	}

	paytmChecksum := PaytmChecksum.GenerateSignature(paytmParams, config.PaymentParams.KEY)
	verifyChecksum := PaytmChecksum.VerifySignature(paytmParams, config.PaymentParams.KEY, paytmChecksum)
	fmt.Printf("->GenerateSignature Returns: %s\n", paytmChecksum)
	fmt.Println(verifyChecksum)

	formFields := ""
	for key, value := range paytmParams {
		formFields += `<input type="hidden" name="` + key + `" value="` + value + `">`
	}
	formFields += `<input type="hidden" name="CHECKSUMHASH" value="` + paytmChecksum + `">`

	fmt.Fprintf(w, `<html><head><title>Merchant Checkout Page</title></head>
					<body><center><h1>Please do not refresh this page...</h1></center>
					<form method="post" action="`+config.PaymentParams.TXNURL+`" name="f1">`+formFields+`</form>
					<script type="text/javascript">document.f1.submit();</script>
					</body></html>`)

}

//CallBackHandler is
func CallBackHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	html := `<body style="justify-content:center;display:flex;"><div>
			<style>td,tr,table{padding:5px;border:1px solid black;}table{border-collapse: collapse;}body{margin-left:50px;margin-top:30px;}button{height:80px;margin:10px 30px;padding:5px 10px;width:150px;}</style>`
	html += `<b>Callback Response</b><br><table>`
	postData := make(map[string]string)
	for key, value := range r.Form {
		html += "<tr><td>" + key + "</td><td>" + value[0] + "</td></tr>"
		postData[key] = value[0]
	}
	html += "</table><br/><br/>"
	checksumhash := postData["CHECKSUMHASH"]
	// delete post_data.CHECKSUMHASH;
	verifyChecksum := PaytmChecksum.VerifySignature(postData, config.PaymentParams.KEY, checksumhash)
	if verifyChecksum == true {
		html += "<b>Checksum Result</b> => True"
	} else {
		html += "<b>Checksum Result</b> => False"
	}
	html += "<br/><br/>"

	// Send Server-to-Server request to verify Order Status
	Body := map[string]string{
		"MID":     postData["MID"],
		"ORDERID": postData["ORDERID"],
	}
	checksum := PaytmChecksum.GenerateSignature(Body, config.PaymentParams.KEY)
	var jsonStr = []byte(`JsonData={"MID":"` + config.PaymentParams.MID + `","ORDERID":"` + postData["ORDERID"] + `","CHECKSUMHASH":"` + checksum + `"}`)
	resp, err := http.Post(config.PaymentParams.TXNSTATUSURL, "application/json", bytes.NewBuffer(jsonStr))
	check(err)
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	var ans Result
	json.Unmarshal(body, &ans)
	fmt.Println("response Body:", ans)

	html += `<b>Validation Response</b><br><table style="width:1130px">`
	v := reflect.ValueOf(ans)
	for i := 0; i < v.NumField(); i++ {
		html += "<tr><td>" + v.Type().Field(i).Name + "</td><td>" + v.Field(i).Interface().(string) + "</td></tr>"
	}
	html += "</table>"
	if ans.RESPCODE == "01" {
		fmt.Println("\n---Transaction Successfull---\n")
		sqlStatement := `
		INSERT INTO Process (TXNID, BANKTXNID, ORDERID, TXNAMOUNT,STATUS,TXNTYPE,GATEWAYNAME,RESPCODE,RESPMSG,BANKNAME,MID,PAYMENTMODE,REFUNDAMT,TXNDATE)
		VALUES ($1, $2, $3, $4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`
		_, err = database.Exec(sqlStatement, ans.TXNID, ans.BANKTXNID, ans.ORDERID, ans.TXNAMOUNT, ans.STATUS, ans.TXNTYPE, ans.GATEWAYNAME, ans.RESPCODE, ans.RESPMSG, ans.BANKNAME, ans.MID, ans.PAYMENTMODE, ans.REFUNDAMT, ans.TXNDATE)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Println("\n---" + ans.STATUS + " : " + ans.RESPMSG + "---\n")
	}
	html += `</div><div><button onclick="show()">Show All Transactions</button><button onclick="pay()">Return To Payment</button></div>`
	html += `<script type="text/javascript">function show(){window.location.replace("http://localhost:3000/db");}`
	html += `function pay(){window.location.replace("http://localhost:3000/start");}</script></body>`

	fmt.Fprintf(w, html)
}

//DbHandler is ...
func DbHandler(w http.ResponseWriter, r *http.Request) {
	var (
		txnid     string
		orderId   string
		txnAmount string
		status    string
		respMsg   string
		paymtMode string
		txnDate   string
	)
	rows, err := database.Query("select TXNID,ORDERID,TXNAMOUNT,STATUS,RESPMSG,PAYMENTMODE,TXNDATE from Process ORDER BY TXNDATE DESC")
	if err != nil {
		check(err)
	}
	defer rows.Close()
	html := `<body style="display:flex;justify-content:center"><div><br/><h3>All Transactions :</h3><br/>`
	html += `<style>table,tr,td,th{border:1px solid black}table{border-collapse: collapse;}td,th{padding:5px 10px;}</style><table>`
	html += `<tr><th>TXNID</th><th>ORDERID</th><th>TXNAMOUNT</th><th>STATUS</th><th>RESPMSG</th><th>PAYMENTMODE</th><th>TXNDATE</th></tr>`
	for rows.Next() {
		err := rows.Scan(&txnid, &orderId, &txnAmount, &status, &respMsg, &paymtMode, &txnDate)
		if err != nil {
			check(err)
		}
		html += `<tr><td>` + txnid + `</td><td>` + orderId + `</td><td>` + txnAmount + `</td><td>` + status + `</td><td>` + respMsg + `</td><td>` + paymtMode + `</td><td>` + txnDate + `</td></tr>`
	}
	html += `</table></div></body>`
	fmt.Fprint(w, html)
}

func main() {
	/* Configration File */
	file, err := os.Open("config.yml")
	check(err)
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	check(err)

	err = yaml.Unmarshal(content, &config)
	check(err)
	fmt.Println(config)

	/* DataBase Connection */
	port, err := strconv.ParseInt(config.DataBase.Port, 10, 64)
	check(err)
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		config.DataBase.Host, port, config.DataBase.User, config.DataBase.Password, config.DataBase.Dbname)
	db, err := sql.Open("postgres", psqlInfo)
	check(err)
	fmt.Println("Successfully connected!")

	/* creating table */
	stmt, err := db.Prepare(`CREATE Table if not exists Process(
		TXNID varchar(50),
		BANKTXNID  varchar(50),
		ORDERID  varchar(50),
		TXNAMOUNT  varchar(50),
		STATUS  varchar(50),
		TXNTYPE  varchar(50),
		GATEWAYNAME  varchar(50),
		RESPCODE  varchar(50),
		RESPMSG  varchar(50),
		BANKNAME  varchar(50),
		MID  varchar(50),
		PAYMENTMODE  varchar(50),
		REFUNDAMT  varchar(50),
		TXNDATE  varchar(50)
	);`)
	check(err)
	_, err = stmt.Exec()
	if err != nil {
		fmt.Println(err.Error())
	} else {
		fmt.Println("Table created successfully..")
	}
	database = db
	defer db.Close()

	http.HandleFunc("/start", PaymentHandler)
	http.HandleFunc("/pay", IndexHandler)
	http.HandleFunc("/callback", CallBackHandler)
	http.HandleFunc("/db", DbHandler)
	http.ListenAndServe(config.PaymentParams.Port, nil)
}
