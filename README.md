# PoC for Compass

Learnings:

Generated SDK's seem to be the way to go for the client: https://buf.build/nativeconnect/api/sdks/main

- buf code generation tool is cool, and the way to manage protos in future, although that is used more for when you'r writing / serving up protos for others to consume,
just for a client no need to go down that path.

- Do no use the connectrpc version of SDK, use the grpc/go version

- You'll need a reference to the v1 models for request/response: import v1 "buf.build/gen/go/nativeconnect/api/protocolbuffers/go/nativeconnect/api/v1"

- add the token in the context for authenticating