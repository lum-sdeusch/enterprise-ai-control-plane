# Vendored: agentgateway ExtMCP protocol

`ext_mcp.proto` is copied verbatim from
`https://github.com/agentgateway/agentgateway/blob/main/crates/protos/proto/ext_mcp.proto`
(Apache-2.0, license text in `LICENSE.agentgateway`). It defines the gRPC hook agentgateway
calls for every MCP request and response phase configured under
`AgentgatewayPolicy.spec.backend.mcp.guardrails.processors[].remote`.

Regenerate the Go bindings with `make proto` (buf, protoc-gen-go, protoc-gen-go-grpc). Re-vendor
when upgrading agentgateway and run `buf breaking` against the previous copy.
