package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/aws/session"
	"time"
	"encoding/json"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"os"
	"net/http"
	"text/template"
	"log"
	"io"
	"io/ioutil"
	"strconv"
)

type Configuration struct {
	Amazon struct {
			   AccountNumber     		string 	`json:"accountNumber"`
			   Endpoint          		string 	`json:"endpoint"`
			   QueueURL          		string 	`json:"queueUrl"`
			   Region            		string 	`json:"region"`
			   InventoryFactsQueueName 	string 	`json:"inventoryFactsQueueName"`
			   InventoryPoolsQueueName 	string 	`json:"inventoryPoolsQueueName"`
		   } 									`json:"amazon"`
	App struct {
			   NumDefinitionRoutines 	int 	`json:"numDefinitionRoutines"`
			   NumFactsRoutines      	int 	`json:"numFactsRoutines"`
		   } 									`json:"app"`
	Mongo struct {
			   DB              			string 	`json:"dB"`
			   FactsCollection 			string 	`json:"factsCollection"`
			   HostAndPort     			string 	`json:"hostAndPort"`
			   PoolsCollection 			string 	`json:"poolsCollection"`
		   } 									`json:"mongo"`
}


type InventoryPoolsDefinitionMessage struct {
	InventoryPools []struct {
		Brand   			string 			`json:"brand"`
		Country 			[]struct {
			CountryCode 	string   		`json:"countryCode"`
			Regions     	[]string 		`json:"regions"`
		} 									`json:"country"`
		ID   				string 			`json:"id"`
		Type 				string 			`json:"type"`
	} 										`json:"inventoryPools"`
}


type InventoryPoolFact struct {
	Brand     				string 			`json:"brand"`
	DocType   				string 			`json:"docType"`
	Pool      				string 			`json:"pool"`
	ProductID 				string 			`json:"productId"`
	Availability    		string 			`json:"availability"`
	BackOrderLevel  		int    			`json:"backOrderLevel"`
	Backorderable   		string 			`json:"backorderable"`
	ShipmentDate    		int    			`json:"shipmentDate"`
	SiteID          		string 			`json:"siteId"`
	SkuID           		string 			`json:"skuId"`
	StockLevel      		int    			`json:"stockLevel"`
	StoreStockLevel 		int    			`json:"storeStockLevel"`
}

type InventoryPoolsFacts struct {
	Brand     				string 			`json:"brand"`
	DocType   				string 			`json:"docType"`
	Pool      				string 			`json:"pool"`
	ProductID 				string 			`json:"productId"`
	Skus      				[]struct {
		Availability    	string 			`json:"availability"`
		BackOrderLevel  	int    			`json:"backOrderLevel"`
		Backorderable   	string 			`json:"backorderable"`
		ShipmentDate    	int    			`json:"shipmentDate"`
		SiteID          	string 			`json:"siteId"`
		SkuID           	string 			`json:"skuId"`
		StockLevel      	int    			`json:"stockLevel"`
		StoreStockLevel 	int    			`json:"storeStockLevel"`
	} 										`json:"skus"`
}

type Page struct {
	Title 					string
	ReceivedDefMsgs 		string
	ReceivedFactsMsgs 		string
	StoredDefMsgs 			string
	StoredFactsMsgs 		string
}


var svc 			*sqs.SQS
var url				string
var mongo_session 	*mgo.Session
var config			Configuration

var pools_definition_messages_received_from_queue int64
var inventory_pools_facts_messages_received_from_queue int64

var pools_definition_messages_stored_in_mongo int64
var inventory_pools_facts_messages_stored_in_mongo int64


func init() {
	// Load Config File
	file, e := ioutil.ReadFile("./config.json")
	if e != nil {
		log.Printf("File error: %v\n", e)
		os.Exit(1)
	}
	log.Printf("%s\n", string(file))

	var config Configuration
	json.Unmarshal(file, &config)

	// Setup connections
	if config.Amazon.Endpoint != "" {
		svc = sqs.New(session.New(), &aws.Config{Endpoint: aws.String("http://localhost:9324"), Region: aws.String(config.Amazon.Region)})
		url = config.Amazon.QueueURL
	} else {
		svc = sqs.New(session.New(), &aws.Config{Region: aws.String(config.Amazon.Region) })
		url = config.Amazon.QueueURL + config.Amazon.AccountNumber + "/"
	}
}

func startGoRoutines() {
	for i:=0;i<config.App.NumDefinitionRoutines;i++ {
		c := make(chan string)
		go updateMongoInventoryPoolDefinition(c)
		go processInventoryPoolDefinition(c)
	}

	for i:=0;i<config.App.NumFactsRoutines;i++ {
		c := make(chan string)
		go updateMongoInventoryPoolFacts(c)
		go processInventoryPoolFacts(c)
	}
}


/*
 *
 *	Program entry point creates connection to SQS and Mongo then pool SQS for messages
 *
 */
func main() {
	counter = 0
	pools_definition_messages_received_from_queue = 0
	pools_definition_messages_stored_in_mongo = 0
	t := time.Now()
	log.Println(t.Format("2006-01-02T15:04:05.999999999Z07:00"))

	file, _ := os.Open("config.json")
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&config)
	if err != nil {
		log.Println("error:", err)
	}
	log.Printf("%+v\n", config)


	startGoRoutines()
	startWebServer()
}




/* ---------------------------------------------------------------------------------------------------------------- */
/* Web Server */

var templates = template.Must(template.ParseFiles("index.html"))

func startWebServer() {
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/stopserver", stopserverHandler)
	log.Printf("About to listen on 10443. Go to https://127.0.0.1:10443/")
	err := http.ListenAndServeTLS(":10443", "cert.pem", "key.pem", nil)
	if err != nil {
		log.Printf("Web Server cannot start: %v\n", err)
	}
}

func stopserverHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Quit signal received from web interface (/stopserver), exiting...")
	io.WriteString(w, "Quit signal received from web interface (/stopserver), exiting...")
	os.Exit(0)
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	p := new(Page)
	p.Title = "Statistics"
	p.ReceivedDefMsgs = strconv.FormatInt(pools_definition_messages_received_from_queue, 10)
	p.ReceivedFactsMsgs = strconv.FormatInt(inventory_pools_facts_messages_received_from_queue, 10)
	p.StoredDefMsgs = strconv.FormatInt(pools_definition_messages_stored_in_mongo, 10)
	p.StoredFactsMsgs = strconv.FormatInt(inventory_pools_facts_messages_stored_in_mongo, 10)
	renderTemplate(w, "index", p)
}


/* ---------------------------------------------------------------------------------------------------------------- */
/* Amazon SQS */

func getSQSMessage(svc *sqs.SQS, queueUrl string) []string {
	var response []string
	params := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueUrl), // Required
		MaxNumberOfMessages: aws.Int64(10),
	}
	resp, err := svc.ReceiveMessage(params)

	if err != nil {
		//log.Println(queueUrl)
		//log.Println("-->> " + err.Error())
		return response
	}

	if len(resp.Messages) > 0 {
		for _, msg := range resp.Messages {
			message := *msg.Body
			delParams := &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueUrl),                        // Required
				ReceiptHandle: aws.String(*msg.ReceiptHandle), // Required
			}
			svc.DeleteMessage(delParams)
			response = append(response, message)
		}
		return response
	}
	return response
}

/* ---------------------------------------------------------------------------------------------------------------- */
/* Amazon pool Def Processing */

func processInventoryPoolDefinition(c chan string) {
	log.Println("processInventory")
	inv_url := url + config.Amazon.InventoryPoolsQueueName

	for {
		messages := getSQSMessage(svc, inv_url)
		if(len(messages) > 0) {
			//log.Printf("%s\n", message)
			for _,message := range messages  {
				pools_definition_messages_received_from_queue++
				c <- message
			}
		}
	}
}

/* ---------------------------------------------------------------------------------------------------------------- */
/* Mongo pool Def Processing */

var counter int

func ProcessPoolDefMsg(session *mgo.Session, poolDefStr string) {
	var PoolMessage InventoryPoolsDefinitionMessage
	json.Unmarshal([]byte(poolDefStr), &PoolMessage)

	c := mongo_session.DB(config.Mongo.DB).C(config.Mongo.PoolsCollection)

	upsertdata := bson.M{ "$set": PoolMessage.InventoryPools[0]}
	_ , err2 := c.UpsertId( PoolMessage.InventoryPools[0].ID, upsertdata )
	if err2 != nil {
		log.Println("Error: ", err2)
	}
	pools_definition_messages_stored_in_mongo++
}

func updateMongoInventoryPoolDefinition(c chan string) {
	log.Println("updateMongo")
	var err error
	mongo_session, err = mgo.Dial(config.Mongo.HostAndPort);
	if err != nil {
		log.Println("Error: ", err)
		return
	}
	defer mongo_session.Close()
	mongo_session.SetMode(mgo.Monotonic, true)


	for {
		message := <- c
		ProcessPoolDefMsg(mongo_session, message)
		counter++
		if(counter % 1000 == 0) {
			counter = 0;
			t := time.Now()
			log.Println(t.Format("2006-01-02T15:04:05.999999999Z07:00"))
		}
	}
}

/* ---------------------------------------------------------------------------------------------------------------- */
/* Amazon pool Facts Processing */

func processInventoryPoolFacts(c chan string) {
	log.Println("processInventoryFacts")
	facts_url := url + config.Amazon.InventoryFactsQueueName

	for {
		messages := getSQSMessage(svc, facts_url)
		if(len(messages) > 0) {
			for _,message := range messages  {
				inventory_pools_facts_messages_received_from_queue++
				c <- message
			}
		}
	}
}



func ProcessPoolfactsMsg(session *mgo.Session, poolDefStr string) {
	var PoolFacts []InventoryPoolsFacts
	json.Unmarshal([]byte(poolDefStr), &PoolFacts)

	c := mongo_session.DB(config.Mongo.DB).C(config.Mongo.FactsCollection)

	if len(PoolFacts) > 0 {
		for i:=0;i<len(PoolFacts);i++ {
			for j:=0;j<len(PoolFacts[i].Skus);j++ {
				p := InventoryPoolFact{ DocType: PoolFacts[i].DocType, Brand: PoolFacts[i].Brand,
					Pool : PoolFacts[i].Pool, ProductID: PoolFacts[i].ProductID, SkuID: PoolFacts[i].Skus[j].SkuID,
					SiteID: PoolFacts[i].Skus[j].SiteID, StockLevel: PoolFacts[i].Skus[j].StockLevel,
					StoreStockLevel: PoolFacts[i].Skus[j].StoreStockLevel, Availability: PoolFacts[i].Skus[j].Availability,
					BackOrderLevel: PoolFacts[i].Skus[j].BackOrderLevel,  Backorderable: PoolFacts[i].Skus[j].Backorderable,
					ShipmentDate: PoolFacts[i].Skus[j].ShipmentDate }

				upsertdata := bson.M{ "$set": p}
				_ , err2 := c.UpsertId( p.Brand+p.Pool+p.ProductID+p.SkuID, upsertdata )
				if err2 != nil {
					log.Println("Error: ", err2)
				}
			}
		}
		inventory_pools_facts_messages_stored_in_mongo++
	}
}

/* ---------------------------------------------------------------------------------------------------------------- */
/* Mongo pool Facts Processing */

func updateMongoInventoryPoolFacts(c chan string) {
	var err error
	mongo_session, err = mgo.Dial(config.Mongo.HostAndPort);
	if err != nil {
		log.Println("Error: ", err)
		return
	}
	defer mongo_session.Close()
	mongo_session.SetMode(mgo.Monotonic, true)


	for {
		message := <- c
		ProcessPoolfactsMsg(mongo_session, message)
		counter++
		if(counter % 1000 == 0) {
			counter = 0;
			t := time.Now()
			log.Println(t.Format("2006-01-02T15:04:05.999999999Z07:00"))
		}
	}
}
