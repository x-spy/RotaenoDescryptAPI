package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {

	http.HandleFunc("/decryptAndSaveGameData", decryptAndSaveApiHandler)
	http.HandleFunc("/decryptGameData", decryptApiHandler)
	http.HandleFunc("/getGameData", getGameDataApiHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type decryptRequestData struct {
	ObjectID string `json:"object-id"`
	SaveData string `json:"save-data"`
}

func getGameDataApiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is accepted", http.StatusMethodNotAllowed)
		return
	}

	objectID, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	executableFile, err := os.Executable()
	if err != nil {
		http.Error(w, "Error retrieving executable path: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rootPath := filepath.Dir(executableFile)
	gameDataPath := filepath.Join(rootPath, "savedGameData", string(objectID)+".txt")

	if _, err := os.Stat(gameDataPath); os.IsNotExist(err) {
		http.Error(w, "No data saved for this objectID.", http.StatusNotFound)
		return
	}

	gameData, err := os.ReadFile(gameDataPath)
	if err != nil {
		http.Error(w, "Error reading saved game data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, string(gameData)); err != nil {
		log.Println("Error writing response: ", err)
	}
}

func decryptAndSaveApiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is accepted", http.StatusMethodNotAllowed)
		return
	}

	objectID, gameData, err := decryptFromRequest(&w, r)
	if err != nil {
		fmt.Println(err)
		return
	}

	executableFile, err := os.Executable()
	if err != nil {
		http.Error(w, "Error retrieving executable path: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rootPath := filepath.Dir(executableFile)
	savedGameDataDir := filepath.Join(rootPath, "savedGameData")
	if err := os.MkdirAll(savedGameDataDir, 0755); err != nil {
		http.Error(w, "Error creating directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	gameDataPath := filepath.Join(savedGameDataDir, objectID+".txt")
	file, err := os.Create(gameDataPath)
	if err != nil {
		http.Error(w, "Error creating file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			fmt.Println("Failed to close file stream.", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}(file)

	if _, err := file.Write(gameData); err != nil {
		http.Error(w, "Error writing to file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, `Game data saved successfully.`); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func decryptApiHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is accepted", http.StatusMethodNotAllowed)
		return
	}

	_, gameData, err := decryptFromRequest(&w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println(string(gameData))

	w.WriteHeader(http.StatusOK)
	_, err = fmt.Fprint(w, string(gameData))
	if err != nil {
		return
	}
}

func decryptFromRequest(w *http.ResponseWriter, r *http.Request) (string, []byte, error) {
	body, err := io.ReadAll(r.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			print(err)
		}
	}(r.Body)

	var data decryptRequestData
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(*w, "Error parsing JSON", http.StatusBadRequest)
		return "", nil, fmt.Errorf("failed to unmarshal JSON")
	}

	saveDataEncrypted, err := base64.StdEncoding.DecodeString(data.SaveData)
	if err != nil {
		http.Error(*w, "Bad base64 string", http.StatusBadRequest)
		return data.ObjectID, nil, fmt.Errorf("bad base64 string")
	}

	keyString := data.ObjectID + "areyoureadyiamlady"
	key := sha256.Sum256([]byte(keyString))

	saveData, err := rotaenoDecrypt(saveDataEncrypted, key[:])
	if err != nil {
		return "", nil, err
	}

	return data.ObjectID, saveData, nil
}
