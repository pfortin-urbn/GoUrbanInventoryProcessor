# URBN Go Inventory Processor

The URBN GO Inventory Processor is a golang app that will listen on Amazon SQS Queues for inventory Pool Definition 
and Inventory Fact Messages and process them into mongo.

##QuickStart

* Install go on your system
* Clone the repository
* Check that the config file is what you want
* use the command: "go run ./inventory_processor.go" to start the application

##Config

Configuration is done via a JSON file called config.json (sample below).  The config can be reloaded at any 
time using the web interface. and is stored in the following GoLang struct

<pre>
type Configuration struct {
	Amazon struct {
        AccountNumber     		        string 	`json:"accountNumber"`
        Endpoint          		        string 	`json:"endpoint"`
        QueueURL          		        string 	`json:"queueUrl"`
        Region            		        string 	`json:"region"`
        InventoryFactsQueueName 	    string 	`json:"inventoryFactsQueueName"`
        InventoryPoolsQueueName 	    string 	`json:"inventoryPoolsQueueName"`
    } 									        `json:"amazon"`
	App struct {
        NumDefinitionRoutines 	        int 	`json:"numDefinitionRoutines"`
        NumFactsRoutines      	        int 	`json:"numFactsRoutines"`
    } 									`json:"app"`
	Mongo struct {
        DB              			    string 	`json:"dB"`
        FactsCollection 			    string 	`json:"factsCollection"`
        HostAndPort     			    string 	`json:"hostAndPort"`
        PoolsCollection 			    string 	`json:"poolsCollection"`
    } 									        `json:"mongo"`
}
</pre>

Amazon:

* **Amazon.Endpoint**
	* Only used for ElasticMQ - "http://localhost:9324"
* **Amazon.AccountNumber (Remenber the leading and trailing '/')**
	* ElasticMQ - "/queue/"
	* Amazon - "/98XXXXXXX86/"
* **Amazon.Region**
	* always set to "us-east-1" for now (even with ElasticMQ)
* **Amazon.QueueUrl**
 	* ElasticMQ - "http://localhost:9324"
	* Amazon - "https://sqs.us-east-1.amazonaws.com"
* **Amazon.InventoryPoolsQueueName** - Inventory Pools Queue Name
* **Amazon.InventoryFactsQueueName** - Inventory Facts Queue Name

Mongo:

* **Mongo.HostAndPort** - (i.e.: "localhost:27017")
* **Mongo.DB** - DB Name - 
* **Mongo.PoolsCollection** - Inventory Definition Collection
* **Mongo.FactsCollection** - Inventory Pools Collection

Misc:

* **App.NumDefinitionRoutines** - number of goroutines that will process inventory definitions simultaneously.
* **App.NumFactsRoutines** - number of goroutines that will process inventory facts simultaneously




