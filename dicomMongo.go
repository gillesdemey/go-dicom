package dicom

import (
	"log"
	"sync"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func stringInSlice(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

// Consumes DicomMessage and outputs bson channel of preselected tags
// as mongoDB fields
func (di *DicomFile) MongoFields(in <-chan DicomMessage, done *sync.WaitGroup, bsonOut chan bson.M, tags []string) <-chan DicomMessage {

	out := make(chan DicomMessage)
	waitMsg := make(chan bool)

	done.Add(1)
	go func() {

		// always insert SOP Instance UID
		tags = append(tags, "00080018")

		var fields bson.M
		fields = make(bson.M)

		for dcmMsg := range in {
			flatTag := dcmMsg.msg.getTagFlat()

			if stringInSlice(dcmMsg.msg.getTagFlat(), tags) {
				var values []interface{}
				for _, val := range dcmMsg.msg.Value {
					values = append(values, val)
				}
				fields[flatTag] = values
			}

			out <- DicomMessage{dcmMsg.msg, waitMsg}
			<-waitMsg
			dcmMsg.wait <- true
		}

		bsonOut <- fields

		close(out)
		done.Done()
	}()
	return out

}

// Generates mongo indexes based on preselected tags and
// input bson channel and in mongoDB collection, and also uploads
// the dicom file to mongo.
func (di *DicomFile) MongoInsert(dialInfo *mgo.DialInfo, collectionName string, bsonChan chan bson.M, idxs []string, filedata []byte) {

	go func() {

		// always insert SOP Instance UID
		idxs = append(idxs, "00080018")
		session, err := mgo.DialWithInfo(dialInfo)
		if err != nil {
			panic(err)
		}
		defer session.Close()

		// Optional. Switch the session to a monotonic behavior.
		session.SetMode(mgo.Monotonic, true)

		db := session.DB(dialInfo.Database)
		collection := db.C(collectionName)

		for _, idx := range idxs {

			// Creates Indexes
			// SOP Instance UID index ensures uniqueness
			index := mgo.Index{
				Key:        []string{idx},
				Unique:     idx == "00080018",
				DropDups:   idx != "00080018",
				Background: true,
				Sparse:     true,
			}
			err := collection.EnsureIndex(index)
			if err != nil {
				log.Println(idx, err)
			}
		}

		var b bson.M
		b = <-bsonChan

		uid := b["00080018"].([]interface{})[0].(string)

		q, err := collection.Find(bson.M{"00080018": uid}).Count()
		if err != nil {
			log.Println("Find.Count:", err)
		}
		if q == 0 {
			log.Println("-> mongo:Insert", uid)
			err = collection.Insert(b)
			if err != nil {
				log.Println("Insert:", err)
			}

			gridFile, err := db.GridFS(collectionName).Create(uid + ".dcm")
			gridFile.SetContentType("Application/dicom")
			if err != nil {
				log.Println("GridFS:Create", err)
			}

			// Write the file to the Mongodb Gridfs instance
			n, err := gridFile.Write(filedata)
			if err != nil {
				log.Println("GridFS:Create", err)
			} else {
				log.Println("GridFS:Create", n)
			}

			// Close the file
			err = gridFile.Close()
			if err != nil {
				log.Println("GridFS:Close", err)
			}

		} else {
			log.Println("-> mongo:Exists", uid)
		}
	}()
}
