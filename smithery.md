> ## Documentation Index
> Fetch the complete documentation index at: https://smithery.ai/docs/llms.txt
> Use this file to discover all available pages before exploring further.

# Connect to MCPs

> Manage MCP connections with a simple REST API - OAuth, tokens, and sessions handled for you.

**Smithery Connect** is Smithery's managed service for connecting to MCP servers. Instead of implementing the MCP protocol directly, handling OAuth flows, and managing credentials yourself, Smithery Connect provides a simple REST interface that handles all of this for you.

## Why Smithery Connect?

Smithery Connect lets you add MCP server integrations to your app without managing the complexity yourself:

* **Zero OAuth configuration** — No redirect URIs, client IDs, or secrets to configure. Smithery maintains OAuth apps for popular integrations.
* **Automatic token refresh** — Tokens refresh automatically before expiry. If a refresh fails, the connection status changes to `auth_required`.
* **Secure credential storage** — Credentials are encrypted and write-only. They can be used to make requests but never read back.
* **Stateless for you** — Smithery Connect manages connection lifecycle. Make requests without worrying about reconnects, keepalives, or session state.
* **Scoped service tokens** — Mint short-lived tokens for browser or mobile clients to call tools directly, scoped to specific users and namespaces.

## Quick Start

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    # 1. Log in to Smithery
    smithery auth login

    # 2. Connect to the Exa search server
    smithery mcp add https://server.smithery.ai/exa --id exa

    # 3. List available tools
    smithery tool list exa

    # 4. Call a tool
    smithery tool call exa search '{"query": "latest news about MCP"}'
    ```
  </Tab>

  <Tab title="AI SDK" icon="sparkles">
    ```bash  theme={null}
    npm install @smithery/api @ai-sdk/mcp ai @ai-sdk/anthropic
    ```

    ```typescript  theme={null}
    import { createMCPClient } from '@ai-sdk/mcp';
    import { generateText } from 'ai';
    import { anthropic } from '@ai-sdk/anthropic';
    import { createConnection } from '@smithery/api/mcp';

    const { transport } = await createConnection({
      mcpUrl: 'https://exa.run.tools',
    });

    const mcpClient = await createMCPClient({ transport });
    const tools = await mcpClient.tools();

    const { text } = await generateText({
      model: anthropic('claude-sonnet-4-20250514'),
      tools,
      prompt: 'Search for the latest news about MCP.',
    });

    await mcpClient.close();
    ```
  </Tab>

  <Tab title="MCP TypeScript SDK" icon="braces">
    ```bash  theme={null}
    npm install @smithery/api @modelcontextprotocol/sdk
    ```

    ```typescript  theme={null}
    import { Client } from '@modelcontextprotocol/sdk/client/index.js';
    import { createConnection } from '@smithery/api/mcp';

    const { transport } = await createConnection({
      mcpUrl: 'https://exa.run.tools',
    });

    // Connect using the MCP SDK Client
    const mcpClient = new Client({ name: 'my-app', version: '1.0.0' });
    await mcpClient.connect(transport);

    // Use the MCP SDK's ergonomic API
    const { tools } = await mcpClient.listTools();
    const result = await mcpClient.callTool({
      name: 'search',
      arguments: { query: 'latest news about MCP' }
    });
    ```
  </Tab>
</Tabs>

## Servers with Configuration

Some MCP servers require configuration like API keys or project IDs. How you pass each config value depends on the server's schema — some values go as **headers** (typically API keys), while others go as **query parameters** in the MCP URL.

Check the server's page on [smithery.ai](https://smithery.ai) to see what configuration it requires and where each value should go.

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    # Add a server with config (API key as header, project ID as query param)
    smithery mcp add \
      '@browserbasehq/mcp-browserbase?browserbaseProjectId=your-project-id' \
      --id my-browserbase \
      --headers '{"browserbaseApiKey": "your-browserbase-api-key"}'
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    import Smithery from '@smithery/api';
    import { createConnection } from '@smithery/api/mcp';

    const smithery = new Smithery();

    // 1. Create a connection with the server's config
    //    - API keys go in `headers`
    //    - Other config goes as query params in `mcpUrl`
    const conn = await smithery.connections.set('my-browserbase', {
      namespace: 'my-app',
      mcpUrl: 'https://browserbase.run.tools?browserbaseProjectId=your-project-id',
      headers: {
        'browserbaseApiKey': 'your-browserbase-api-key',
      },
    });
    // conn.status.state === "connected" — ready to use immediately

    // 2. Get a transport for the connection
    const { transport } = await createConnection({
      client: smithery,
      namespace: 'my-app',
      connectionId: conn.connectionId,
    });
    ```
  </Tab>

  <Tab title="cURL" icon="globe">
    ```bash  theme={null}
    curl -X POST "https://api.smithery.ai/connect/my-app" \
      -H "Authorization: Bearer $SMITHERY_API_KEY" \
      -H "Content-Type: application/json" \
      -d '{
        "mcpUrl": "https://browserbase.run.tools?browserbaseProjectId=your-project-id",
        "headers": {
          "browserbaseApiKey": "your-browserbase-api-key"
        }
      }'
    ```
  </Tab>
</Tabs>

Unlike OAuth-based servers that return `auth_required`, servers configured with API keys return `connected` immediately.

<Note>
  Each server's config schema specifies whether a field is passed as a **header** or **query parameter** via the `x-from` metadata. See [Session Configuration](/build/session-config) for details on how servers declare their config transport.
</Note>

## Multi-User Setup

When your agent serves multiple users, you'll need to track which connections belong to which user. Use the `metadata` field to associate connections with your users, then filter by metadata to retrieve a specific user's connections.

### 1. Create a Connection for a User

When a user wants to connect an integration (e.g., GitHub), create a connection with their `userId` in metadata:

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    smithery mcp add https://server.smithery.ai/github \
      --id user-123-github \
      --name "GitHub" \
      --metadata '{"userId": "user-123"}'

    # If OAuth is required, the CLI outputs the auth URL:
    # → auth_required
    # → https://auth.smithery.ai/...
    # Visit the URL to authorize
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    const conn = await smithery.connections.set('user-123-github', {
      namespace: 'my-app',
      mcpUrl: 'https://github.run.tools',
      name: 'GitHub',
      metadata: { userId: 'user-123' }
    });

    if (conn.status === 'auth_required') {
      // Redirect user to complete OAuth
      redirect(conn.authorizationUrl);
    }
    ```
  </Tab>
</Tabs>

### 2. List a User's Connections

When your agent needs to know what tools are available for a user, list their connections:

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    smithery mcp list --metadata '{"userId": "user-123"}'
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    const connections = await smithery.connections.list('my-app', {
      metadata: { userId: 'user-123' }
    });

    // Show the user their connected integrations
    for (const conn of connections.data) {
      console.log(`${conn.name}: ${conn.status}`);
    }
    ```
  </Tab>
</Tabs>

### 3. Use Tools Across Connections

Create MCP clients for each connection and aggregate their tools:

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    # List tools for a connection
    smithery tool list user-123-github

    # Call a tool
    smithery tool call user-123-github search_repositories \
      '{"query": "mcp"}'
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    const allTools = [];

    for (const conn of connections.data) {
      if (conn.status === 'connected') {
        const { transport } = await createConnection({
          namespace: 'my-app',
          connectionId: conn.connectionId,
        });
        const client = await createMCPClient({ transport });
        allTools.push(...await client.tools());
      }
    }

    // Now your agent has tools from all the user's connected integrations
    const { text } = await generateText({
      model: anthropic('claude-sonnet-4-20250514'),
      tools: allTools,
      prompt: userMessage,
    });
    ```
  </Tab>
</Tabs>

## Core Concepts

### Namespaces

A namespace is a globally unique identifier that groups your connections. Create one namespace per application or environment (e.g., `my-app`, `my-app-staging`). If you don't specify a namespace, the SDK uses your first existing namespace or creates one automatically.

### Connections

A connection is a long-lived session to an MCP server that persists until terminated. Each connection:

* Has a `connectionId` (developer-defined or auto-generated)
* Stores credentials securely (write-only—credentials can never be read back, only used to execute requests)
* Can include custom `metadata` for filtering (e.g., `userId` to associate connections with your users)
* Returns `serverInfo` with the MCP server's name and version

<Accordion title="createConnection Options">
  | Option         | Type        | Description                                                                                  |
  | -------------- | ----------- | -------------------------------------------------------------------------------------------- |
  | `client`       | `Smithery?` | The Smithery client instance. If not provided, auto-created using `SMITHERY_API_KEY` env var |
  | `mcpUrl`       | `string?`   | The MCP server URL. Required when `connectionId` is not provided                             |
  | `namespace`    | `string?`   | If omitted, uses first existing namespace or creates one                                     |
  | `connectionId` | `string?`   | If omitted, an ID is auto-generated                                                          |

  Returns a `SmitheryConnection` with `{ transport, connectionId, url }`.
  Throws `SmitheryAuthorizationError` if OAuth authorization is required (see [Handling Authorization](#handling-authorization)).
</Accordion>

### Connection Status

When you create or retrieve a connection, it includes a `status` field:

| Status          | Description                                                                  |
| --------------- | ---------------------------------------------------------------------------- |
| `connected`     | Connection is ready to use                                                   |
| `auth_required` | OAuth authorization needed. Includes `authorizationUrl` to redirect the user |
| `error`         | Connection failed. Includes error `message`                                  |

### Authentication

Smithery Connect uses two authentication methods:

| Token             | Use Case                | Access                                |
| ----------------- | ----------------------- | ------------------------------------- |
| **API Key**       | Backend only            | Full namespace access                 |
| **Service Token** | Browser, mobile, agents | Scoped access to specific connections |

Get your API key from [smithery.ai/account/api-keys](https://smithery.ai/account/api-keys).

<Warning>
  Never expose your API key to untrusted clients or agents. Use scoped service tokens for browser and mobile apps.
</Warning>

### OAuth Flow

When an MCP server requires OAuth:

1. Create a connection—the response status will be `auth_required` with an `authorizationUrl`
2. Redirect the user to `authorizationUrl`
3. User completes OAuth with the upstream provider (e.g., GitHub)
4. User is redirected back to your app
5. The connection is now ready—subsequent requests will succeed

You don't need to register OAuth apps, configure redirect URIs, or handle token exchange. Smithery manages the OAuth relationship with upstream providers and stores credentials securely on your behalf.

### Handling Authorization

When using the SDK, `createConnection()` throws a `SmitheryAuthorizationError` if the MCP server requires OAuth. The error contains the URL to redirect the user to and the connection ID to retry with after authorization completes.

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    # Connect to a server that requires OAuth
    smithery mcp add https://server.smithery.ai/github

    # If auth is required, the CLI outputs the authorization URL:
    # → auth_required
    # → https://auth.smithery.ai/...
    # → connection_id: abc-123-github
    # Visit the URL to authorize, then retry
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    import { createConnection, SmitheryAuthorizationError } from '@smithery/api/mcp';

    try {
      const { transport } = await createConnection({
        mcpUrl: 'https://github.run.tools',
      });

      // Connection succeeded — use transport as normal
      const mcpClient = await createMCPClient({ transport });
      const tools = await mcpClient.tools();
    } catch (error) {
      if (error instanceof SmitheryAuthorizationError) {
        // Redirect the user to complete OAuth
        // error.authorizationUrl — where to send the user
        // error.connectionId — save this to retry after auth completes
        redirect(error.authorizationUrl);
      }
      throw error;
    }
    ```
  </Tab>
</Tabs>

After the user completes authorization and returns to your app, retry with the saved `connectionId`:

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    # After authorization, the connection is ready
    smithery tool list abc-123-github
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    const { transport } = await createConnection({
      connectionId: savedConnectionId,
    });

    const mcpClient = await createMCPClient({ transport });
    const tools = await mcpClient.tools();
    ```
  </Tab>
</Tabs>

## Service Tokens

Service tokens let you safely use the Smithery Connect from browsers, mobile apps, and AI agents without exposing your API key. Your backend mints a scoped token, then your client uses it to call tools directly.

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    smithery auth token --policy '[{
      "namespaces": "my-app",
      "resources": "connections",
      "operations": ["read", "execute"],
      "metadata": { "userId": "user-123" },
      "ttl": "1h"
    }]'
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    // Create a token scoped to a specific user's connections
    const { token } = await smithery.tokens.create({
      policy: [
        {
          namespaces: 'my-app',
          resources: 'connections',
          operations: ['read', 'execute'],
          metadata: { userId: 'user-123' },
          ttl: '1h',
        },
      ],
    })

    // Send `token` to your client — safe for browser use
    ```
  </Tab>

  <Tab title="cURL" icon="globe">
    ```bash  theme={null}
    curl -X POST https://api.smithery.ai/tokens \
      -H "Authorization: Bearer $SMITHERY_API_KEY" \
      -H "Content-Type: application/json" \
      -d '{
        "policy": [
          {
            "namespaces": "my-app",
            "resources": "connections",
            "operations": ["read", "execute"],
            "metadata": { "userId": "user-123" },
            "ttl": "1h"
          }
        ]
      }'
    ```
  </Tab>
</Tabs>

This token can only access connections in `my-app` where `metadata.userId` matches `user-123`. Initialize the client with the token:

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    # Use the scoped token to call tools
    SMITHERY_API_KEY=$TOKEN smithery tool list user-123-github
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    const smithery = new Smithery({ apiKey: token })

    const { transport } = await createConnection({
      client: smithery,
      namespace: 'my-app',
      connectionId: 'user-123-github',
    })
    ```
  </Tab>
</Tabs>

<Note>
  Need workspace-level scoping, read-only tokens, or token narrowing? See [Token Scoping](/use/token-scoping) for the full guide.
</Note>

## Advanced

For full API documentation, see the [Smithery Connect Reference](/api-reference/connect/).

### Direct MCP Calls

Call MCP methods directly via the Streamable HTTP endpoint:

<Tabs>
  <Tab title="CLI" icon="terminal">
    ```bash  theme={null}
    # List available tools
    smithery tool list user-123-github

    # Call a tool
    smithery tool call user-123-github search_repositories \
      '{"query": "mcp"}'
    ```
  </Tab>

  <Tab title="TypeScript" icon="braces">
    ```typescript  theme={null}
    // List available tools
    const tools = await smithery.connections.mcp.call('user-123-github', {
      namespace: 'my-app',
      method: 'tools/list',
      params: {}
    });

    // Call a tool
    const result = await smithery.connections.mcp.call('user-123-github', {
      namespace: 'my-app',
      method: 'tools/call',
      params: {
        name: 'search_repositories',
        arguments: { query: 'mcp' }
      }
    });
    ```
  </Tab>

  <Tab title="cURL" icon="globe">
    ```bash  theme={null}
    # List available tools
    curl -X POST https://api.smithery.ai/connect/my-app/user-123-github/mcp \
      -H "Authorization: Bearer $SMITHERY_API_KEY" \
      -H "Content-Type: application/json" \
      -d '{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": {}}'

    # Call a tool
    curl -X POST https://api.smithery.ai/connect/my-app/user-123-github/mcp \
      -H "Authorization: Bearer $SMITHERY_API_KEY" \
      -H "Content-Type: application/json" \
      -d '{
        "jsonrpc": "2.0",
        "id": 2,
        "method": "tools/call",
        "params": {
          "name": "search_repositories",
          "arguments": { "query": "mcp" }
        }
      }'
    ```
  </Tab>
</Tabs>
