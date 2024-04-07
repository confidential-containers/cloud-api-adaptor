package test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

type kbsport string

var keys map[string][]byte

func (p kbsport) getRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		keyMaterial, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad Body", http.StatusBadRequest)
			return
		}
		keys[r.URL.Path] = keyMaterial
		return
	}
	if r.Method == "GET" {
		keyMaterial, ok := keys[r.URL.Path]
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		_, err := w.Write(keyMaterial)
		if err != nil {
			http.Error(w, "cant write response", http.StatusInternalServerError)
			return
		}
		return
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}

func KBSServer(port string) {
	keys = map[string][]byte{}

	p := kbsport(port)
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.getRoot)
	s := &http.Server{
		Addr:    "127.0.0.1:" + port,
		Handler: mux,
	}

	go func() {
		err := s.ListenAndServe()
		fmt.Printf("ListenAndServe Error %v\n", err)
	}()
}

type GetKeyClient struct {
	port string
}

func NewGetKeyClient(port string) *GetKeyClient {
	return &GetKeyClient{
		port: port,
	}
}

// getKey uses kbs-client to obtain keys such as pp-sid/privateKey, sshclient/publicKey
func (c *GetKeyClient) GetKey(key string) (data []byte, err error) {
	url := fmt.Sprintf("http://127.0.0.1:%s/kbs/v0/resource/default/%s", c.port, key)

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				log.Printf("getKey client.DialContext() addr: %s", addr)
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("getKey %s client.Get err: %w", key, err)
	}
	defer resp.Body.Close()

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("getKey %s io.ReadAll err - %w", key, err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("getKey %s client.Get received error code of : %d - %s", key, resp.StatusCode, string(data))
	}

	log.Printf("getKey %s statusCode %d success", key, resp.StatusCode)
	return
}
