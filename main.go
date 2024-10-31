package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/ssoready/ssoready-go"
	ssoreadyclient "github.com/ssoready/ssoready-go/client"
	ssoreadyoption "github.com/ssoready/ssoready-go/option"
)

// This demo just renders plain old HTML with no client-side JavaScript. This is
// only to keep the demo minimal. SSOReady works with any frontend stack or
// framework you use.
//
// This demo keeps the HTML minimal to keep things as simple as possible here.
var indexTemplate = template.Must(template.New("").Parse(`
<!doctype html>
<html>
	<head>
		<title>SAML Demo App using SSOReady</title>
		<script src="https://cdn.tailwindcss.com"></script>
	</head>
	<body>
		<main class="grid min-h-screen place-items-center py-32 px-8">
			<div class="text-center">
				<h1 class="mt-4 text-balance text-5xl font-semibold tracking-tight text-gray-900 sm:text-7xl">
					Hello, {{ or .Email "logged-out user" }}!
				</h1>
				<p class="mt-6 text-pretty text-lg font-medium text-gray-500 sm:text-xl/8">
					This is a SAML demo app, built using SSOReady.
				</p>

				<!-- submitting this form makes the user's browser do a GET /saml-redirect?email=... -->
				<form method="get" action="/saml-redirect" class="mt-10 max-w-lg mx-auto">
					<div class="flex gap-x-4 items-center">
						<label for="email-address" class="sr-only">Email address</label>
						<input id="email-address" name="email" class="min-w-0 flex-auto rounded-md border-0 px-3.5 py-2 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6" value="john.doe@example.com" placeholder="john.doe@example.com">
						<button type="submit" class="flex-none rounded-md bg-indigo-600 px-3.5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600">
							Log in with SAML
						</button>
						<a href="/logout" class="px-3.5 py-2.5 text-sm font-semibold text-gray-900">
							Sign out
						</a>
					</div>
					<p class="mt-4 text-sm leading-6 text-gray-900">
						(Try any @example.com or @example.org email address.)
					</p>
				</form>
			</div>
		</main>
	</body>
</html>
`))

func main() {
	mux := http.NewServeMux()

	// Do not hard-code or leak your SSOReady API key in production!
	//
	// In production, instead you should configure a secret SSOREADY_API_KEY
	// environment variable. The SSOReady SDK automatically loads an API key
	// from SSOREADY_API_KEY.
	//
	// This key is hard-coded here for the convenience of logging into a test
	// app, which is hard-coded to run on http://localhost:8080. It's only
	// because of this very specific set of constraints that it's acceptable to
	// hard-code and publicly leak this API key.
	ssoreadyClient := ssoreadyclient.NewClient(ssoreadyoption.WithAPIKey("ssoready_sk_4w96zfjul38drbitw1hbd3sqv"))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		var email string
		cookie, _ := r.Cookie("email")
		if cookie != nil {
			email = cookie.Value
		}

		if err := indexTemplate.Execute(w, map[string]string{"Email": email}); err != nil {
			panic(err)
		}
	})

	// This is the page users visit when they click on the "Log out" link in this
	// demo app. It just resets the `email` cookie.
	//
	// SSOReady doesn't impose any constraints on how your app's sessions work.
	mux.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:   "email",
			MaxAge: -1,
		})
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// This is the page users visit when they submit the "Log in with SAML" form
	// in this demo app.
	mux.HandleFunc("GET /saml-redirect", func(w http.ResponseWriter, r *http.Request) {
		// converts "john.doe@example.com" into "example.com".
		_, domain, _ := strings.Cut(r.URL.Query().Get("email"), "@")

		// To start a SAML login, you need to redirect your user to their employer's
		// particular Identity Provider. This is called "initiating" the SAML login.
		//
		// Use `SAML.GetSAMLRedirectURL` to initiate a SAML login.
		getRedirectURLRes, err := ssoreadyClient.SAML.GetSAMLRedirectURL(r.Context(), &ssoready.GetSAMLRedirectURLRequest{
			// OrganizationExternalID is how you tell SSOReady which company's
			// identity provider you want to redirect to.
			//
			// In this demo, we identify companies using their domain.
			OrganizationExternalID: &domain,
		})
		if err != nil {
			panic(err)
		}

		// `SAML.GetSAMLRedirectURL` returns a struct like this:
		//
		// GetSAMLRedirectURLResponse{RedirectURL: "https://..."}
		//
		// To initiate a SAML login, you redirect the user to that RedirectURL.
		http.Redirect(w, r, *getRedirectURLRes.RedirectURL, http.StatusFound)
	})

	// This is the page SSOReady redirects your users to when they've
	// successfully logged in with SAML.
	mux.HandleFunc("GET /ssoready-callback", func(w http.ResponseWriter, r *http.Request) {
		// SSOReady gives you a one-time SAML access code under
		// ?saml_access_code=saml_access_code_... in the callback URL's query
		// parameters.
		//
		// You redeem that SAML access code using `SAML.RedeemSAMLAccessCode`,
		// which gives you back the user's email address. Then, it's your job to
		// log the user in as that email.
		samlAccessCode := r.URL.Query().Get("saml_access_code")
		redeemRes, err := ssoreadyClient.SAML.RedeemSAMLAccessCode(r.Context(), &ssoready.RedeemSAMLAccessCodeRequest{
			SAMLAccessCode: &samlAccessCode,
		})
		if err != nil {
			panic(err)
		}

		// SSOReady works with any stack or session technology you already use.
		//
		// As a proof-of-concept, this demo just writes the email as a
		// plaintext, unsigned cookie. Don't do this in production.
		http.SetCookie(w, &http.Cookie{
			Name:  "email",
			Value: *redeemRes.Email,
		})
		http.Redirect(w, r, "/", http.StatusFound)
	})

	fmt.Println("listening on http://localhost:8080")
	if err := http.ListenAndServe("localhost:8080", mux); err != nil {
		panic(err)
	}
}
