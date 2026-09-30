(function () {
  "use strict";

  const script = document.currentScript;
  if (!script || !script.dataset.apiBase) {
    return;
  }

  const apiBaseURL = new URL(script.dataset.apiBase)
    .toString()
    .replace(/\/+$/, "");

  const session = {
    apiBaseURL,
    csrfToken: null,
    csrfPromise: null,
    latestRequestID: "",
    statusElement: null,
    requestIDElement: null,
    copyButton: null,

    setStatus(message) {
      if (this.statusElement) {
        this.statusElement.textContent = message;
      }
    },

    recordRequestID(requestID) {
      if (!requestID) {
        return;
      }

      this.latestRequestID = requestID;
      if (this.requestIDElement) {
        this.requestIDElement.textContent = requestID;
      }
      if (this.copyButton) {
        this.copyButton.disabled = false;
      }
    },

    loadCsrfToken(force) {
      if (force) {
        this.csrfToken = null;
      }
      if (this.csrfToken) {
        return Promise.resolve(this.csrfToken);
      }
      if (this.csrfPromise) {
        return this.csrfPromise;
      }

      const self = this;
      this.csrfPromise = window
        .fetch(this.apiBaseURL + "/auth/me", {
          credentials: "include",
          headers: { Accept: "application/json" },
        })
        .then(function (response) {
          self.recordRequestID(response.headers.get("X-Request-ID"));
          if (response.status === 401) {
            self.csrfToken = null;
            self.setStatus(
              "No active API session. Sign in with Google, then refresh the session.",
            );
            return null;
          }
          if (!response.ok) {
            throw new Error(
              "Could not read the session (HTTP " + response.status + ").",
            );
          }

          return response.json();
        })
        .then(function (payload) {
          if (payload === null) {
            return null;
          }
          if (
            !payload ||
            typeof payload.csrf_token !== "string" ||
            !payload.csrf_token
          ) {
            throw new Error(
              "The session response did not include a CSRF token.",
            );
          }

          self.csrfToken = payload.csrf_token;
          self.setStatus(
            "Session ready. CSRF token is held in memory and added to mutations.",
          );
          return self.csrfToken;
        })
        .catch(function (error) {
          self.setStatus("Session check failed: " + error.message);
          throw error;
        })
        .finally(function () {
          self.csrfPromise = null;
        });

      return this.csrfPromise;
    },
  };

  window.__swaggerSession = session;

  function requestInterceptor(request) {
    const target = new URL(request.url, window.location.href);
    const apiBase = new URL(session.apiBaseURL);
    const apiPath = apiBase.pathname.replace(/\/+$/, "");
    const isAPIRequest =
      target.origin === apiBase.origin &&
      (target.pathname === apiPath ||
        target.pathname.startsWith(apiPath + "/"));
    const method = String(request.method || "GET").toUpperCase();
    const isMutation = ["POST", "PUT", "PATCH", "DELETE"].includes(method);

    if (!isAPIRequest || !isMutation) {
      return request;
    }

    return session.loadCsrfToken(false).then(function (token) {
      if (token) {
        request.headers = request.headers || {};
        if (typeof request.headers.set === "function") {
          request.headers.set("X-CSRF-Token", token);
        } else {
          request.headers["X-CSRF-Token"] = token;
        }
      }

      return request;
    });
  }

  function responseInterceptor(response) {
    const responseURL =
      response.url || (response.request && response.request.url) || "";
    let responsePath = "";
    try {
      const target = new URL(responseURL, window.location.href);
      const apiBase = new URL(session.apiBaseURL);
      const apiPath = apiBase.pathname.replace(/\/+$/, "");
      const isAPIResponse =
        target.origin === apiBase.origin &&
        (target.pathname === apiPath ||
          target.pathname.startsWith(apiPath + "/"));
      if (!isAPIResponse) {
        return response;
      }
      responsePath = target.pathname;
    } catch (_) {
      return response;
    }

    const headers = response.headers || {};
    const requestID =
      typeof headers.get === "function"
        ? headers.get("X-Request-ID")
        : headers["X-Request-ID"] || headers["x-request-id"];
    session.recordRequestID(requestID);

    if (
      responsePath.endsWith("/auth/me") &&
      response.status === 200 &&
      response.body &&
      response.body.csrf_token
    ) {
      session.csrfToken = response.body.csrf_token;
      session.setStatus(
        "Session ready. CSRF token is held in memory and added to mutations.",
      );
    }

    if (response.status === 401) {
      session.csrfToken = null;
      session.setStatus(
        "Session missing or expired. Sign in with Google, then refresh the session.",
      );
    }

    if (response.status === 403) {
      session.csrfToken = null;
      session.setStatus(
        "Request forbidden. Check access, Origin, or CSRF; the token will refresh on retry.",
      );
    }

    if (response.status === 204 && responsePath.endsWith("/auth/logout")) {
      session.csrfToken = null;
      session.setStatus(
        "Signed out. Sign in again before sending protected requests.",
      );
    }

    return response;
  }

  function makeButton(label) {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = label;
    button.style.cssText =
      "padding:6px 10px;border:1px solid #7b8797;border-radius:4px;background:white;color:#1b1b1b;cursor:pointer";
    return button;
  }

  function onComplete() {
    if (document.getElementById("swagger-session-tools")) {
      return;
    }

    const panel = document.createElement("aside");
    panel.id = "swagger-session-tools";
    panel.setAttribute("aria-label", "Browser session and request information");
    panel.style.cssText =
      "max-width:1460px;margin:0 auto;padding:14px 20px;border-bottom:1px solid #d8dee8;background:#f7f9fc;color:#1b1b1b;font:14px/1.5 sans-serif;display:flex;gap:12px;align-items:center;flex-wrap:wrap";

    const instructions = document.createElement("span");
    instructions.textContent =
      "The browser sends its HttpOnly API cookie. Sign in here; Swagger reads /auth/me and adds CSRF to mutations.";
    panel.appendChild(instructions);

    const signInButton = makeButton("Sign in with Google");
    signInButton.disabled = !script.dataset.loginClientId;
    signInButton.addEventListener("click", function () {
      session.csrfToken = null;
      const login = new URL(session.apiBaseURL + "/auth/login");
      login.searchParams.set("client_id", script.dataset.loginClientId);
      window.open(login.toString(), "_blank", "noopener");
      session.setStatus(
        "Sign-in opened in another tab. After returning to the app, refresh the session here.",
      );
    });
    panel.appendChild(signInButton);

    const refreshButton = makeButton("Refresh session");
    refreshButton.addEventListener("click", function () {
      session.loadCsrfToken(true).catch(function () {});
    });
    panel.appendChild(refreshButton);

    session.statusElement = document.createElement("span");
    session.statusElement.setAttribute("role", "status");
    session.statusElement.textContent =
      "Session not checked. Sign in or refresh the session before sending mutations.";
    panel.appendChild(session.statusElement);

    const requestIDLabel = document.createElement("span");
    requestIDLabel.textContent = "Latest request ID:";
    panel.appendChild(requestIDLabel);

    session.requestIDElement = document.createElement("code");
    session.requestIDElement.textContent = "none";
    panel.appendChild(session.requestIDElement);

    session.copyButton = makeButton("Copy ID");
    session.copyButton.disabled = true;
    session.copyButton.addEventListener("click", async function () {
      if (!session.latestRequestID) {
        return;
      }

      try {
        await navigator.clipboard.writeText(session.latestRequestID);
        session.setStatus("Request ID copied.");
      } catch (_) {
        session.setStatus("Copy failed. Select the request ID to copy it.");
      }
    });
    panel.appendChild(session.copyButton);

    const swaggerRoot = document.getElementById("swagger-ui");
    if (swaggerRoot && swaggerRoot.parentNode) {
      swaggerRoot.parentNode.insertBefore(panel, swaggerRoot);
    }
  }

  window.IntaniaSwaggerSession = {
    requestInterceptor,
    responseInterceptor,
    onComplete,
  };
})();
