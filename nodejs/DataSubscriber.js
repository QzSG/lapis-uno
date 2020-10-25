const MongoClient = require('mongodb').MongoClient;
var mqtt = require('mqtt')
var protobuf = require("protobufjs");

const mongo_uri = "mongodb+srv://undecimus:" + process.env.MONGO_PASS + "@cluster0.cbskb.mongodb.net/sampleDB?retryWrites=true&w=majority";
const mongoClient = new MongoClient(mongo_uri, { useNewUrlParser: true });
// assign the client from MongoClient
mongoClient
  .connect()
  .then(client => {

    console.log("Connected successfully to server");

    const db = mongoClient.db('sampleDB');

    var mqtt_options = {
      clientId: "dashboard-client",
      username: "xilinx",
      password: "undecimus",
      clean: true
    };
    var client = mqtt.connect('mqtts://mqtts.qz.sg', mqtt_options)

    var mqtt_topic = "test/+/data"

    var ReadingMessage = null;

    protobuf.load("../protobuf/reading.proto", function (err, root) {
      if (err)
        throw err;


      ReadingMessage = root.lookupType("Reading");
    })

    client.on('connect', function () {
      client.subscribe(mqtt_topic, function (err) {
        if (!err) {
          console.log("Subscribed to " + mqtt_topic)
        }
      })
    })

    client.on('message', function (topic, message) {
      // message is Buffer
      console.log(topic)
      var reading = ReadingMessage.decode(message);
      reading.topic = topic
      console.log(reading)
      db.collection('readings').insertOne({
        isStartMove: reading.isStartMove,
        clientID: reading.clientID,
        dancerNo: reading.dancerNo,
        accX: reading.accX,
        accY: reading.accY,
        accZ: reading.accZ,
        gyroRoll : reading.gyroRoll,
        gyroPitch : reading.gyroPitch,
        gyroYaw: reading.gyroYaw,
        timeStamp : reading.timeStamp.toString()
      }).then(function (result) {
        console.log(result)
      })
    })

  })
  .catch(error => console.error(error));
// listen for the signal interruption (ctrl-c)
process.on('SIGINT', () => {
  mongoClient.close();
  process.exit();
});
