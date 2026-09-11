// Package rest provides the HTTP driving adapter exposing wallet management endpoints.
// It handles transport decoding and status mapping, delegating logic to the service.
package rest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ging/alexandria/internal/common"
	"github.com/ging/alexandria/internal/ssi-auth/wallet"
)

// WalletRouter is the driving adapter that exposes the wallet use cases over
// HTTP. It chooses status codes and shapes bodies; it decides nothing else.
type WalletRouter struct {
	holder *wallet.Service
}

// NewWalletRouter wires the router onto the wallet service it drives.
func NewWalletRouter(holder *wallet.Service) *WalletRouter {
	return &WalletRouter{holder: holder}
}

// Register mounts the wallet routes under the given parent group.
func (r *WalletRouter) Register(parent *gin.RouterGroup) *gin.RouterGroup {
	walletRouter := parent.Group("/wallet")

	walletRouter.GET("/is-linked", r.isLinked) // ok
	walletRouter.POST("/link", r.link)         // ok

	walletRouter.POST("/keys", r.registerKey)     // ok
	walletRouter.GET("/keys", r.getWalletKeys)    // ok
	walletRouter.DELETE("/keys/:id", r.deleteKey) // ok
	walletRouter.POST("/keys/:id/rotate", r.rotateKey)
	walletRouter.POST("/keys/:id/revoke", r.revokeKey)

	didRouter := walletRouter.Group("/did")
	didRouter.GET("", r.getWalletDid) // ok
	didRouter.GET("/all", r.getAllDids)
	didRouter.POST("", r.registerDid) // ok
	didRouter.GET("/:id", r.getDid)
	didRouter.DELETE("/:id", r.deleteDid) // ok
	didRouter.POST("/:id/default", r.setDefaultDid)
	didRouter.POST("/:id/key/:key_id", r.addKeyToDid)
	didRouter.DELETE("/:id/key/:key_id", r.removeKeyFromDid)
	didRouter.POST("/:id/key/:key_id/default", r.setDefaultKey)
	didRouter.POST("/:id/publish", r.publishDid)
	didRouter.POST("/:id/unpublish", r.unpublishDid)
	didRouter.GET("/:id/state", r.getDidState)
	didRouter.POST("/:id/endpoints", r.addServiceEndpoint)
	didRouter.DELETE("/:id/endpoints/:endpoint_id", r.removeServiceEndpoint)

	walletRouter.DELETE("/credential/:id", r.deleteCredential)
	walletRouter.POST("/credential", r.storeCredential)
	walletRouter.GET("/vcs", r.getWalletCredentials)
	walletRouter.GET("/info", r.getWalletInfo)

	walletRouter.POST("/oid4vci", r.processOid4vci)
	walletRouter.POST("/oid4vp", r.processOid4vp)

	dcpRouter := walletRouter.Group("/dcp")
	dcpRouter.POST("/request", r.requestDcpCredential)
	dcpRouter.GET("/request/:id", r.getDcpRequestStatus)

	partRouter := walletRouter.Group("/participants")
	partRouter.POST("", r.createParticipant)
	partRouter.GET("/:id", r.getParticipant)
	partRouter.POST("/:id/state", r.setParticipantState)
	partRouter.POST("/:id/token", r.regenerateParticipantToken)
	partRouter.PUT("/:id/token", r.updateParticipantToken)

	return walletRouter
}

// RegisterWellKnown mounts the routes that must answer from the root of the
// host rather than from under the API prefix, because a did:web resolver looks
// for them at a fixed path.
func (r *WalletRouter) RegisterWellKnown(engine *gin.Engine) {
	engine.GET("/.well-known/did.json", r.getDidDoc)
}

// ===== HTTP handlers =========================================================

func (r *WalletRouter) link(c *gin.Context) {
	did, err := r.holder.Link(c.Request.Context())
	if err != nil {
		respondError(c, err)

		return
	}

	c.JSON(http.StatusOK, newDidResp(did))
}

func (r *WalletRouter) isLinked(c *gin.Context) {
	if !r.holder.IsLinked(c.Request.Context()) {
		c.Status(http.StatusNotFound)

		return
	}

	c.Status(http.StatusOK)
}

func (r *WalletRouter) registerKey(c *gin.Context) {
	var req registerKeyReq

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	keyMaterial := req.rawKey()
	key, err := r.holder.RegisterKey(
		c.Request.Context(),
		keyMaterial,
		&req.Alias,
		req.ID,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newKeyResp(key))
}

func (r *WalletRouter) deleteKey(c *gin.Context) {
	keyID := c.Params.ByName("id")
	err := r.holder.DeleteKey(c, keyID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (r *WalletRouter) getWalletKeys(c *gin.Context) {
	keys, err := r.holder.Keys(c)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newKeyResps(keys))
}

func (r *WalletRouter) registerDid(c *gin.Context) {
	var req registerDidReq

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	builder, err := req.Builder.toDomain()
	if err != nil {
		respondError(c, err)
		return
	}

	services, err := servicesToDomain(req.Service)
	if err != nil {
		respondError(c, err)
		return
	}

	did, err := r.holder.RegisterDid(
		c.Request.Context(),
		builder,
		req.Keys,
		req.Alias,
		services,
	)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newDidResp(did))
}

func (r *WalletRouter) getWalletDid(c *gin.Context) {
	id, err := r.holder.Did(c.Request.Context())
	if err != nil {
		respondError(c, err)

		return
	}

	c.JSON(http.StatusOK, didIDResp{Did: id})
}

// getAllDids renders every DID the wallet holds.
func (r *WalletRouter) getAllDids(c *gin.Context) {
	dids, err := r.holder.GetAllDids(c.Request.Context())
	if err != nil {
		respondError(c, err)

		return
	}

	c.JSON(http.StatusOK, newDidResps(dids))
}

func (r *WalletRouter) getDidDoc(c *gin.Context) {
	doc, err := r.holder.DidDoc(c.Request.Context())
	if err != nil {
		respondError(c, err)

		return
	}
	c.JSON(http.StatusOK, &doc)
}

// getDid renders a single DID record, or 404 when there is none.
func (r *WalletRouter) getDid(c *gin.Context) {
	didID := c.Params.ByName("id")
	did, err := r.holder.GetDidByID(c.Request.Context(), didID)
	if err != nil {
		respondError(c, err)

		return
	}

	c.JSON(http.StatusOK, newDidResp(did))
}

func (r *WalletRouter) deleteDid(c *gin.Context) {
	didID := c.Params.ByName("id")
	err := r.holder.DeleteDid(c, didID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

// setDefaultDid promotes a DID to be the wallet active identity.
func (r *WalletRouter) setDefaultDid(c *gin.Context) {
	didID := c.Params.ByName("id")
	did, err := r.holder.SetDefaultDid(c, didID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newDidResp(did))
}

// addKeyToDid binds a key into the verification methods of a DID.
func (r *WalletRouter) addKeyToDid(c *gin.Context) {
	didID := c.Params.ByName("id")
	keyID := c.Params.ByName("key_id")
	did, err := r.holder.AddKeyToDid(c, didID, keyID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newDidResp(did))
}

// removeKeyFromDid unbinds a key from the verification methods of a DID.
func (r *WalletRouter) removeKeyFromDid(c *gin.Context) {
	didID := c.Params.ByName("id")
	keyID := c.Params.ByName("key_id")
	did, err := r.holder.RemoveKeyFromDid(c, didID, keyID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newDidResp(did))
}

// setDefaultKey promotes a key to be the default verification method of a DID.
func (r *WalletRouter) setDefaultKey(c *gin.Context) {
	didID := c.Params.ByName("id")
	keyID := c.Params.ByName("key_id")
	did, err := r.holder.SetDefaultKey(c, didID, keyID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newDidResp(did))
}

func (r *WalletRouter) deleteCredential(c *gin.Context) {
	credentialID := c.Params.ByName("id")
	err := r.holder.DeleteCredential(c, credentialID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

func (r *WalletRouter) getWalletCredentials(c *gin.Context) {
	vcType := c.Query("type")
	var credentials []wallet.Credential
	var err error
	if vcType != "" {
		credentials, err = r.holder.GetCredentialsByType(c.Request.Context(), vcType)
	} else {
		credentials, err = r.holder.Credentials(c.Request.Context())
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newCredentialResps(credentials))
}

func (r *WalletRouter) getWalletInfo(c *gin.Context) {
	walletInfo, err := r.holder.Info(c)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, newWalletInfoRest(walletInfo))
}

func (r *WalletRouter) processOid4vci(c *gin.Context) {
	var req oidcURIReq

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	if err := r.holder.ProcessOid4vci(c.Request.Context(), req.URI); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

func (r *WalletRouter) processOid4vp(c *gin.Context) {
	var req oidcURIReq

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	if err := r.holder.ProcessOid4vp(c.Request.Context(), req.URI); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

func (r *WalletRouter) rotateKey(c *gin.Context) {
	keyID := c.Params.ByName("id")
	var req rotateKeyReq
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(c, err)
		return
	}

	var duration time.Duration
	if req.Duration != "" {
		d, err := time.ParseDuration(req.Duration)
		if err != nil {
			respondError(c, common.Invalid("duration", "must be a valid duration (e.g. 1h, 24h)"))
			return
		}
		duration = d
	}

	key, err := r.holder.RotateKey(c.Request.Context(), keyID, duration)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newKeyResp(key))
}

func (r *WalletRouter) revokeKey(c *gin.Context) {
	keyID := c.Params.ByName("id")
	if err := r.holder.RevokeKey(c.Request.Context(), keyID); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusAccepted)
}

func (r *WalletRouter) publishDid(c *gin.Context) {
	didID := c.Params.ByName("id")
	st, err := r.holder.PublishDid(c.Request.Context(), didID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, didStateResp{
		Did:   st.Did,
		State: string(st.State),
	})
}

func (r *WalletRouter) unpublishDid(c *gin.Context) {
	didID := c.Params.ByName("id")
	st, err := r.holder.UnpublishDid(c.Request.Context(), didID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, didStateResp{
		Did:   st.Did,
		State: string(st.State),
	})
}

func (r *WalletRouter) getDidState(c *gin.Context) {
	didID := c.Params.ByName("id")
	st, err := r.holder.GetDidState(c.Request.Context(), didID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, didStateResp{
		Did:   st.Did,
		State: string(st.State),
	})
}

func (r *WalletRouter) addServiceEndpoint(c *gin.Context) {
	didID := c.Params.ByName("id")
	var req serviceEndpointReq
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	did, err := r.holder.AddServiceEndpoint(c.Request.Context(), didID, wallet.ServiceEndpointPlan{
		ID:   req.ID,
		Type: req.Type,
		URL:  req.URL,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newDidResp(did))
}

func (r *WalletRouter) removeServiceEndpoint(c *gin.Context) {
	didID := c.Params.ByName("id")
	endpointID := c.Params.ByName("endpoint_id")
	did, err := r.holder.RemoveServiceEndpoint(c.Request.Context(), didID, endpointID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newDidResp(did))
}

func (r *WalletRouter) storeCredential(c *gin.Context) {
	var req storeCredentialReq
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	plan := &wallet.CredentialImportPlan{
		ID:                   req.ID,
		ParticipantContextID: req.ParticipantContextID,
		Format:               req.Format,
		RawVc:                req.RawVc,
		Credential:           req.Credential,
		Payload:              req.Payload,
	}
	if req.VerifiableCredentialContainer != nil {
		plan.VerifiableCredentialContainer = &wallet.CredentialContainerPlan{
			RawVc:      req.VerifiableCredentialContainer.RawVc,
			Format:     req.VerifiableCredentialContainer.Format,
			Credential: req.VerifiableCredentialContainer.Credential,
		}
	}

	cred, err := r.holder.StoreCredential(c.Request.Context(), plan)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newCredentialResp(cred))
}

func (r *WalletRouter) requestDcpCredential(c *gin.Context) {
	var req dcpCredentialRequestReq
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	var creds []wallet.CredentialDescriptor
	for _, cd := range req.Credentials {
		creds = append(creds, wallet.CredentialDescriptor{
			ID:     cd.ID,
			Format: cd.Format,
			Type:   cd.Type,
		})
	}

	reqID, err := r.holder.RequestDcpCredential(c.Request.Context(), &wallet.DcpCredentialRequestPlan{
		IssuerURL:   req.IssuerURL,
		IssuerDid:   req.IssuerDid,
		HolderPid:   req.HolderPid,
		Types:       req.Types,
		Format:      req.Format,
		Credentials: creds,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, dcpRequestResp{RequestID: reqID})
}

func (r *WalletRouter) getDcpRequestStatus(c *gin.Context) {
	reqID := c.Params.ByName("id")
	st, err := r.holder.GetDcpRequestStatus(c.Request.Context(), reqID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dcpStatusResp{
		RequestID: st.RequestID,
		Status:    st.Status,
		Error:     st.Error,
	})
}

func (r *WalletRouter) createParticipant(c *gin.Context) {
	var req createParticipantReq
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	plan := &wallet.ParticipantPlan{
		ID:     req.ID,
		Did:    req.Did,
		Active: req.Active,
	}
	if req.KeyDescriptor != nil {
		plan.KeyDescriptor = &wallet.KeyDescriptor{
			KeyID:              req.KeyDescriptor.KeyID,
			Type:               req.KeyDescriptor.Type,
			PrivateKeyAlias:    req.KeyDescriptor.PrivateKeyAlias,
			KeyGeneratorParams: req.KeyDescriptor.KeyGeneratorParams,
			PublicKeyJwk:       req.KeyDescriptor.PublicKeyJwk,
			PublicKeyPem:       req.KeyDescriptor.PublicKeyPem,
			Properties:         req.KeyDescriptor.Properties,
		}
	}

	p, err := r.holder.CreateParticipant(c.Request.Context(), plan)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, newParticipantResp(p))
}

func (r *WalletRouter) getParticipant(c *gin.Context) {
	participantID := c.Params.ByName("id")
	p, err := r.holder.GetParticipant(c.Request.Context(), participantID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newParticipantResp(p))
}

func (r *WalletRouter) setParticipantState(c *gin.Context) {
	participantID := c.Params.ByName("id")
	var req participantStateReq
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		respondError(c, err)
		return
	}

	p, err := r.holder.SetParticipantState(c.Request.Context(), participantID, req.Active)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, newParticipantResp(p))
}

func (r *WalletRouter) regenerateParticipantToken(c *gin.Context) {
	participantID := c.Params.ByName("id")
	tok, err := r.holder.RegenerateParticipantToken(c.Request.Context(), participantID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, participantTokenResp{Token: tok})
}

type updateTokenReq struct {
	Token string `json:"token" binding:"required"`
}

func (r *WalletRouter) updateParticipantToken(c *gin.Context) {
	participantID := c.Params.ByName("id")
	var req updateTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	if err := r.holder.UpdateParticipantToken(c.Request.Context(), participantID, req.Token); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "token updated in memory",
		"participantId": participantID,
	})
}
