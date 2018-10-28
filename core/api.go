package core

import (
	"archive/tar"
	"compress/gzip"
	"context"
	njwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/textileio/textile-go/ipfs"
	"github.com/textileio/textile-go/jwt"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	uio "gx/ipfs/QmebqVUQQqQFhg74FtQFszUJo22Vpr3e8qBAkvvV4ho9HH/go-ipfs/unixfs/io"
	"io"
	"net/http"
	"strings"
)

// httpApiVersion is the api version
const httpApiVersion = "v0"

// httpApiHost is the instance used by the daemon
var httpApiHost *httpApi

// HttpApi is a limited HTTP API to the cafe service and locally run commands
type httpApi struct {
	addr   string
	server *http.Server
	node   *Textile
}

// StartHttpApi starts the host instance
func (t *Textile) StartHttpApi(addr string) {
	httpApiHost = &httpApi{addr: addr, node: t}
	httpApiHost.Start()
}

// StopHttpApi starts the host instance
func (t *Textile) StopHttpApi() error {
	return httpApiHost.Stop()
}

// Addr returns the api address
func (t *Textile) HttpApiAddr() string {
	return httpApiHost.server.Addr
}

// Start starts the http api
func (c *httpApi) Start() {
	// setup router
	router := gin.Default()
	router.GET("/", func(g *gin.Context) {
		g.JSON(http.StatusOK, gin.H{
			"api_version":  httpApiVersion,
			"node_version": Version,
		})
	})
	router.GET("/health", func(g *gin.Context) {
		g.Writer.WriteHeader(http.StatusNoContent)
	})

	// v0 routes
	v0 := router.Group("/api/v0")
	{
		v0.POST("/pin", c.pin)
	}
	c.server = &http.Server{
		Addr:    c.addr,
		Handler: router,
	}

	// start listening
	errc := make(chan error)
	go func() {
		errc <- c.server.ListenAndServe()
		close(errc)
	}()
	go func() {
		for {
			select {
			case err, ok := <-errc:
				if err != nil && err != http.ErrServerClosed {
					log.Errorf("api error: %s", err)
				}
				if !ok {
					log.Info("api was shutdown")
					return
				}
			}
		}
	}()
	log.Infof("api listening at %s\n", c.server.Addr)
}

// Stop stops the http api
func (c *httpApi) Stop() error {
	ctx, cancel := context.WithCancel(context.Background())
	if err := c.server.Shutdown(ctx); err != nil {
		log.Errorf("error shutting down api: %s", err)
		return err
	}
	cancel()
	return nil
}

// PinResponse is the json response from a pin request
type PinResponse struct {
	Id    string `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

// forbiddenResponse is used for bad tokens
var forbiddenResponse = PinResponse{
	Error: errForbidden,
}

// unauthorizedResponse is used when a token is expired or not present
var unauthorizedResponse = PinResponse{
	Error: errUnauthorized,
}

// pin take raw data or a tarball and pins it to the local ipfs node.
// request must be authenticated with a token
func (c *httpApi) pin(g *gin.Context) {
	if !c.node.Started() {
		g.AbortWithStatusJSON(http.StatusInternalServerError, PinResponse{
			Error: "node is stopped",
		})
		return
	}
	var id *cid.Cid

	// get the auth token
	auth := strings.Split(g.Request.Header.Get("Authorization"), " ")
	if len(auth) < 2 {
		g.AbortWithStatusJSON(http.StatusUnauthorized, unauthorizedResponse)
		return
	}
	token := auth[1]

	// validate token
	proto := string(c.node.cafeService.Protocol())
	if err := jwt.Validate(token, c.verifyKeyFunc, false, proto, nil); err != nil {
		switch err {
		case jwt.ErrNoToken, jwt.ErrExpired:
			g.AbortWithStatusJSON(http.StatusUnauthorized, unauthorizedResponse)
		case jwt.ErrInvalid:
			g.AbortWithStatusJSON(http.StatusForbidden, forbiddenResponse)
		}
		return
	}

	// handle based on content type
	cType := g.Request.Header.Get("Content-Type")
	switch cType {
	case "application/gzip":
		// create a virtual directory for the photo
		dirb := uio.NewDirectory(c.node.Ipfs().DAG)
		// unpack archive
		gr, err := gzip.NewReader(g.Request.Body)
		if err != nil {
			log.Errorf("error creating gzip reader %s", err)
			g.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		tr := tar.NewReader(gr)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Errorf("error getting tar next %s", err)
				g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			switch header.Typeflag {
			case tar.TypeDir:
				log.Error("got nested directory, aborting")
				g.JSON(http.StatusBadRequest, gin.H{"error": "directories are not supported"})
				return
			case tar.TypeReg:
				if _, err := ipfs.AddFileToDirectory(c.node.Ipfs(), dirb, tr, header.Name); err != nil {
					log.Errorf("error adding file to dir %s", err)
					g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			default:
				continue
			}
		}

		// pin the directory
		dir, err := dirb.GetNode()
		if err != nil {
			log.Errorf("error creating dir node %s", err)
			g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := ipfs.PinDirectory(c.node.Ipfs(), dir, []string{}); err != nil {
			log.Errorf("error pinning dir node %s", err)
			g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		id = dir.Cid()

	case "application/octet-stream":
		var err error
		id, err = ipfs.PinData(c.node.Ipfs(), g.Request.Body)
		if err != nil {
			log.Errorf("error pinning raw body %s", err)
			g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
		log.Errorf("got bad content type %s", cType)
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid content-type"})
		return
	}
	hash := id.Hash().B58String()

	log.Debugf("pinned request with content type %s: %s", cType, hash)

	// ship it
	g.JSON(http.StatusCreated, PinResponse{
		Id: hash,
	})
}

// verifyKeyFunc returns the correct key for token verification
func (c *httpApi) verifyKeyFunc(token *njwt.Token) (interface{}, error) {
	if !c.node.Started() {
		return nil, ErrStopped
	}
	return c.node.Ipfs().PrivateKey.GetPublic(), nil
}
