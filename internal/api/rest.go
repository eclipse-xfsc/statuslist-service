package api

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	logger "github.com/sirupsen/logrus"
	ginSwagger "github.com/swaggo/gin-swagger"

	server "github.com/eclipse-xfsc/microservice-core-go/pkg/server"
	"github.com/eclipse-xfsc/statuslist-service/internal/config"
	"github.com/eclipse-xfsc/statuslist-service/internal/database"
	"github.com/gin-gonic/gin"
)

type apienv struct {
	db *database.Database
}

var conf *config.StatusListConfiguration

func (c *apienv) SetSwaggerBasePath(path string) {
}

// SwaggerOptions swagger config options. See https://github.com/swaggo/gin-swagger?tab=readme-ov-file#configuration
func (c *apienv) SwaggerOptions() []func(config *ginSwagger.Config) {
	return make([]func(config *ginSwagger.Config), 0)
}

func handleGetList(ctx *gin.Context) {
	tenantId := ctx.Param("tenantId")
	listId, err := strconv.Atoi(ctx.Param("listId"))

	cty := ctx.ContentType()

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	b, err := db.GetStatusList(ctx, tenantId, listId)
	if err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	_, err = zw.Write(b)

	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err := zw.Close(); err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if cty == "application/json" || cty == "" {

		ctx.JSON(http.StatusOK, gin.H{
			"tenantId": tenantId,
			"listId":   listId,
			"list":     base64.RawStdEncoding.EncodeToString(buf.Bytes()),
		})
		return
	}

	if cty == "statuslist+jwt" || cty == "application/vc+ld+json" {
		key := ctx.Request.Header.Get("X-KEY")
		did := ctx.Request.Header.Get("X-DID")
		namespace := ctx.Request.Header.Get("X-NAMESPACE")
		group := ctx.Request.Header.Get("X-GROUP") //can be ""!
		host := ctx.Request.Header.Get("X-HOST")
		listtype := ctx.Request.Header.Get("X-TYPE")

		if key == "" {
			key = conf.DefaultKey
		}
		if did == "" {
			did = conf.DefaultDid
		}

		if namespace == "" {
			namespace = conf.DefaultNamespace
		}

		if group == "" {
			group = conf.DefaultGroup
		}

		if host == "" {
			host = conf.DefaultHost
		}

		if listtype == "" {
			listtype = conf.DefaultListType
		}

		if cty == "statuslist+jwt" {

			res, err := requestTokenSigning(tenantId, base64.RawStdEncoding.EncodeToString(buf.Bytes()), key, namespace, group, did, host, 1, listId)

			if err != nil {
				ctx.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			ctx.JSON(http.StatusOK, string(res))
			return
		}

		if cty == "application/vc+ld+json" {
			if listtype == "StatusList2021" {
				res, err := handleCredentialSigning2021(tenantId, base64.RawStdEncoding.EncodeToString(buf.Bytes()), key, namespace, group, did, host, strconv.Itoa(listId))
				if err != nil {
					ctx.AbortWithStatus(http.StatusInternalServerError)
					return
				}

				ctx.JSON(http.StatusOK, res)
				return
			}
		}
	}

	ctx.AbortWithStatus(http.StatusBadRequest)
}

func handleCredentialSigning2021(tenantId, statusList, key, namespace, group, did, host, listid string) (map[string]interface{}, error) {

	payload := make(map[string]interface{})

	payload["namespace"] = namespace
	payload["group"] = group
	payload["key"] = key

	credential := make(map[string]interface{})
	credential["@context"] = []string{"https://www.w3.org/2018/credentials/v1", "https://w3id.org/vc/status-list/2021/v1", "https://w3id.org/security/suites/jws-2020/v1"}
	credential["type"] = []string{"VerifiableCredential", "StatusList2021Credential"}
	credential["id"] = host + "/" + listid
	credential["issuer"] = did
	credential["issuanceDate"] = time.Now().UTC().Format(time.RFC3339)
	subject := make(map[string]interface{})
	subject["id"] = host + "/" + listid + "#list"
	subject["type"] = "StatusList2021"
	subject["statusPurpose"] = "revocation"
	subject["encodedList"] = statusList
	credential["credentialSubject"] = subject
	payload["credential"] = credential
	p, err := json.Marshal(payload)

	if err != nil {
		return nil, err
	}

	rep, err := http.DefaultClient.Post(conf.SignerUrl+"/credential/proof", "application/json", bytes.NewBuffer(p))

	if err != nil {
		return nil, err
	}

	respBody, err := io.ReadAll(rep.Body)

	if err != nil {
		return nil, err
	}

	defer rep.Body.Close()

	if rep.StatusCode != http.StatusOK {
		return nil, errors.New("signer service call error. result was: " + string(respBody))
	}

	var r map[string]interface{}
	err = json.Unmarshal(respBody, &r)

	if err != nil {
		return nil, err
	}
	return r, nil

}

func handleRevoke(ctx *gin.Context) {
	tenantId := ctx.Param("tenantId")
	listId, err := strconv.Atoi(ctx.Param("listId"))
	if err != nil {
		logger.Error(err.Error(), "Error parsing listId")
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	index, err := strconv.Atoi(ctx.Param("index"))
	if err != nil {
		logger.Error(err.Error(), "Error parsing index")
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	err = db.RevokeCredentialInSpecifiedList(ctx, tenantId, listId, index)
	if err != nil {
		logger.Error("Error revoking credential", err.Error())
		ctx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"tenantId": tenantId,
		"listId":   listId,
		"index":    index,
		"status":   "revoked",
	})
}

func startRest(c *config.StatusListConfiguration, wg *sync.WaitGroup, db *database.Database) {
	defer wg.Done()
	conf = c
	env := new(apienv)
	env.SetDb(db)

	srv := server.New(env)

	srv.Add(func(tenantsGrp *gin.RouterGroup) {
		grp := tenantsGrp.Group("/status")
		grp.POST("/:listId/revoke/:index", handleRevoke)
		grp.GET("/:listId", handleGetList)
	})

	err := srv.Run(c.ListenPort)
	if err != nil {
		panic(err)
	}
}

func (env *apienv) SetDb(db *database.Database) {
	env.db = db
}

func (env *apienv) IsHealthy() bool {
	return env.db.Ping()
}
