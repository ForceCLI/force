# DPoP-Bound Sessions

`force login --dpop` requests tokens bound to a key pair with OAuth 2.0 Demonstrating Proof of Possession (DPoP, RFC 9449). It works with the browser, device, JWT bearer, and client credentials logins. Salesforce does not support DPoP for the username-password flow, and the SOAP login does not use OAuth.

## Login

force generates a P-256 key pair and sends a DPoP proof signed with it to the token endpoint. For the browser login, the authorize URL also includes `dpop_jkt`, the JWK thumbprint of the public key, so that Salesforce binds the authorization code to the key. Salesforce answers the first token request with a `use_dpop_nonce` error and a nonce in the `DPoP-Nonce` header, and force sends the request again with the nonce in the proof. If the external client app or connected app allows DPoP, Salesforce returns `token_type` `DPoP`, and force saves the private key and the nonce with the session, in the same file as the refresh token. If Salesforce returns a Bearer token instead, force does not save the key and the session works as a Bearer session.

## Requests

Each later request made with a DPoP session carries a new proof signed with the key and including the nonce:

- REST requests (REST, Tooling, Composite, Bulk API 2.0, Apex REST, Connect, SCIM, and the Streaming API used by `force log tail`) send `Authorization: DPoP <token>` and the proof in the `DPoP` header.
- Bulk API 1.0 requests send `Authorization: DPoP <token>` and the proof in the `DPoP` header. Salesforce compares the proof's `htu` with the request URL using the `http` scheme, so force writes `http://` in place of `https://` in `htu` for these requests.
- SOAP requests (Metadata, Partner, Apex) send the token in `SessionHeader` and the proof in a `DpopProofHeader` element of the SOAP envelope.
- Pub/Sub API requests send the token in the `accesstoken` gRPC metadata header without a proof. Salesforce accepts the DPoP-bound token there without one.
- Refresh token requests send the proof in the `DPoP` header.

## Web UI Sessions

The browser login exchanges the authorization code with the `hybrid_auth_code` grant, and DPoP sessions are refreshed with the `hybrid_refresh` grant. Along with the DPoP-bound token, these grants return `ui_sid`, an unbound session ID for the web UI, which `force open` uses with `frontdoor.jsp`, since a browser can't send DPoP proofs. The device, JWT bearer, and client credentials logins have no hybrid grant, so their sessions get a `ui_sid` at the first refresh.

## Nonce Expiration

Salesforce accepts a nonce for 24 hours. When it rejects a proof because the nonce expired (`INVALID_AUTH_HEADER`), force refreshes the session, which gets a new nonce from the token endpoint, and sends the request again.

## Limitations

- `force open` returns an error for a session that has no `ui_sid` yet: a device, JWT bearer, or client credentials login that has not been refreshed.
- The token printed by `force active --session` can't be used by other tools without the key.
