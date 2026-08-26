package api

import (
	"net/http"
	"strings"

	"enygma-server/config"
	"enygma-server/pkg/circuits/burn"
	"enygma-server/pkg/circuits/deposit"
	"enygma-server/pkg/circuits/enygma"
	"enygma-server/pkg/circuits/enygma_fee"
	"enygma-server/pkg/circuits/withdraw"
	"github.com/gin-gonic/gin"
)

// maxRequestBodyBytes bounds every request body read by the proving
// server (Fix M-08: "no MaxBytesReader anywhere; only
// len(req.PublicSignal) > 80 was checked" — and that check ran only
// after the whole body was already read). A genuine proof request is a
// few KB of decimal-string fields; 512KiB is generous headroom over that
// while still bounding the worst case.
const maxRequestBodyBytes = 512 * 1024

func NewServer(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	r.Use(maxBodySize(maxRequestBodyBytes))
	r.Use(requireJSONContentType())

	r.POST("/proof/enygma", enygma.NewHandler(cfg.EnygmaPk, cfg.EnygmaVk))
	r.POST("/proof/enygma_fee", enygma_fee.NewHandler(cfg.EnygmaFeePk, cfg.EnygmaFeeVk))
	// Fix M-16: was six routes (/proof/withdraw/1..6) for one constraint
	// system — see config.Config.WithdrawPk6's doc comment. Only /6 was
	// ever reachable through Enygma.sol's withdraw() (which forces
	// commitmentDeltas.length == DEFAULT_SIZE == 6), so it's the one kept.
	r.POST("/proof/withdraw/6", withdraw.NewHandler(cfg.WithdrawPk6, cfg.WithdrawVk6))
	r.POST("/proof/deposit", deposit.NewHandler(cfg.DepositPk, cfg.DepositVk))
	r.POST("/proof/burn", burn.NewHandler(cfg.BurnPk, cfg.BurnVk))
	return r
}

// maxBodySize wraps every request body in http.MaxBytesReader so a caller
// cannot force the server to buffer an unbounded body into memory before
// gin's JSON binding even runs (Fix M-08).
func maxBodySize(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

// requireJSONContentType rejects any request whose Content-Type is not
// application/json.
//
// Fix M-08: "ShouldBindJSON does not consult Content-Type, so every route
// is drivable from a browser page as a CORS-simple text/plain POST" —
// text/plain, application/x-www-form-urlencoded and multipart/form-data
// are the three CORS-simple content types a cross-origin page can send
// without triggering a preflight (and hence without needing any
// permissive CORS response from this server, which gin.Default() does
// not send). Requiring application/json forces the browser to preflight,
// which this server has no CORS headers to satisfy — so a page on any
// other origin can no longer drive these routes at all, even against a
// loopback-only deployment reachable through a victim's own browser.
func requireJSONContentType() gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := c.GetHeader("Content-Type")
		// Allow an optional charset parameter (e.g. "application/json;
		// charset=utf-8"), matching what real JSON clients commonly send.
		mediaType := strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])
		if !strings.EqualFold(mediaType, "application/json") {
			c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
				"error": "Content-Type must be application/json",
			})
			return
		}
		c.Next()
	}
}
