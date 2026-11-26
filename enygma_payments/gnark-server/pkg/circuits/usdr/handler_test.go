package usdr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"enygma_payments/gnark-server/utils"
	"github.com/gin-gonic/gin"
)

func TestUSDrUsesProveLimiter(t *testing.T) {
	oldLimiter := utils.ProveLimiter
	utils.ProveLimiter = make(chan struct{}, 1)
	if !utils.ProveLimiter.Acquire() {
		t.Fatal("could not reserve the proving slot")
	}
	t.Cleanup(func() {
		utils.ProveLimiter.Release()
		utils.ProveLimiter = oldLimiter
	})

	zeros := []string{"0", "0", "0", "0", "0", "0"}
	commits := [][2]string{{"0", "0"}, {"0", "0"}, {"0", "0"}, {"0", "0"}, {"0", "0"}, {"0", "0"}}
	body, err := json.Marshal(USDrRequest{
		FingerPrintofSharedSecrets: [][]string{zeros, zeros, zeros, zeros, zeros, zeros},
		PublicKey:                  zeros, PreviousCommit: commits, TxCommit: commits,
		BlockNumber: "0", AnonymitySet: zeros, MessageTags: zeros, Nullifier: "0",
		SenderID: "0", SharedSecrets: zeros, SecretKey: "0", PreviousSenderBalance: "0",
		PreviousSenderRandomValue: "0", TxValues: zeros, TxRandomValues: zeros,
		SenderTxValue: "0", DomainId: "0", FeeRecipientKey: "0",
	})
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.POST("/proof/usdr", NewHandler("../../../keys/UsdrPk.key", "../../../keys/UsdrVk.key"))
	req := httptest.NewRequest(http.MethodPost, "/proof/usdr", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d %s while the proving slot was held, want 503", w.Code, w.Body)
	}
}
