package server

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"b612.me/starlog"
	"b612.me/starmap"

	"github.com/Songmu/go-httpdate"
	activitypub "github.com/starainrt/Activity-Relay/ActivityPub"
	keyloader "github.com/starainrt/Activity-Relay/KeyLoader"
	"github.com/starainrt/Activity-Relay/conf"
	"github.com/yukimochi/httpsig"
)

func decodeActivity(request *http.Request) (*activitypub.Activity, *activitypub.Actor, []byte, error) {
	request.Header.Set("Host", request.Host)
	dataLen, _ := strconv.Atoi(request.Header.Get("Content-Length"))
	body := make([]byte, dataLen)
	request.Body.Read(body)
	cfg := starmap.MustGet("config").(conf.RelayConfig)
	if !cfg.SupportAuthorized {
		// Verify HTTPSignature
		verifier, err := httpsig.NewVerifier(request)
		if err != nil {
			return nil, nil, nil, err
		}
		KeyID := verifier.KeyId()
		keyOwnerActor := new(activitypub.Actor)
		err = keyOwnerActor.RetrieveRemoteActor(KeyID, starmap.MustGet("ua").(string), actorCache)
		if err != nil {
			starlog.Debugln("Cannot Verify HTTP Signature", err)
			return nil, nil, nil, err
		}
		PubKey, err := keyloader.ReadPublicKeyRSAfromString(keyOwnerActor.PublicKey.PublicKeyPem)
		if PubKey == nil {
			starlog.Debugln("Public Key is Null", err)
			return nil, nil, nil, errors.New("Failed parse PublicKey from string")
		}
		if err != nil {
			starlog.Debugln("Cannot ReadPublicKeyRSAfromString", err)
			return nil, nil, nil, err
		}
		err = verifier.Verify(PubKey, httpsig.RSA_SHA256)
		if err != nil {
			starlog.Debugln("Cannot Verify ", err)
			return nil, nil, nil, err
		}

		// Verify Digest
		givenDigest := request.Header.Get("Digest")
		hash := sha256.New()
		hash.Write(body)
		b := hash.Sum(nil)
		calcurateDigest := "SHA-256=" + base64.StdEncoding.EncodeToString(b)

		if givenDigest != calcurateDigest {
			return nil, nil, nil, errors.New("Digest header is mismatch")
		}
	}
	// Parse Activity
	var activity activitypub.Activity
	err := json.Unmarshal(body, &activity)
	if err != nil {
		starlog.Errorln("Cannot Parse Activity to Json", err)
		return nil, nil, nil, err
	}

	var remoteActor activitypub.Actor
	if !cfg.SupportAuthorized {
		err = remoteActor.RetrieveRemoteActor(activity.Actor, starmap.MustGet("ua").(string), actorCache)
	} else {
		nilByte := []byte{}
		req, _ := http.NewRequest("GET", activity.Actor, nil)
		err = appendSignature(req, &nilByte, Actor.ID, hostPrivatekey)
		if err != nil {
			starlog.Errorln("Append error", err)
			return nil, nil, nil, err
		}
		err = remoteActor.RetrieveRemoteActorSigned(activity.Actor, req, actorCache)
	}
	if err != nil {
		starlog.Errorln("Cannot Get Remote Actor", activity.Actor, err)
		return nil, nil, nil, err
	}

	return &activity, &remoteActor, body, nil
}

func appendSignature(request *http.Request, body *[]byte, KeyID string, publicKey *rsa.PrivateKey) error {
	hash := sha256.New()
	hash.Write(*body)
	b := hash.Sum(nil)
	request.Header.Set("Content-Type", "application/activity+json")
	request.Header.Set("User-Agent", starmap.MustGet("ua").(string))
	request.Header.Set("Date", httpdate.Time2Str(time.Now()))
	request.Header.Set("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(b))
	request.Header.Set("Host", request.Host)

	signer, _, err := httpsig.NewSigner([]httpsig.Algorithm{httpsig.RSA_SHA256}, []string{httpsig.RequestTarget, "Host", "Date", "Digest", "Content-Type"}, httpsig.Signature)
	if err != nil {
		return err
	}
	err = signer.SignRequest(publicKey, KeyID, request)
	if err != nil {
		return err
	}
	return nil
}
