package test

import (
	"fmt"
	"io"
	"net/http"
)

func HttpClient(dest string) bool {
	fmt.Printf("HttpClient start : %s\n", dest)

	fmt.Printf("HttpClient sending req: %s\n", dest)
	c := http.Client{}
	resp, err := c.Get(dest)
	if err != nil {
		fmt.Printf("HttpClient %s Error %s\n", dest, err)
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("HttpClient %s ReadAll Error %s\n", dest, err)
		return false
	}
	fmt.Printf("HttpClient %s Body : %s\n", dest, body)
	return (resp.StatusCode == 200)
}
