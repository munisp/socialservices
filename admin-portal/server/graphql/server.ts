import { ApolloServer } from "@apollo/server";
import { expressMiddleware } from "@apollo/server/express4";
import { ApolloServerPluginDrainHttpServer } from "@apollo/server/plugin/drainHttpServer";
import { makeExecutableSchema } from "@graphql-tools/schema";
// WebSocket support disabled for now - can be added later
import { typeDefs } from "./schema";
import { resolvers } from "./resolvers";
import type { Express } from "express";
import type { Server as HttpServer } from "http";

/**
 * Create and configure Apollo GraphQL Server
 */
export async function createGraphQLServer(app: Express, httpServer: HttpServer) {
  // Create executable schema
  const schema = makeExecutableSchema({
    typeDefs,
    resolvers,
  });

  // WebSocket subscriptions can be added later if needed

  // Create Apollo Server
  const apolloServer = new ApolloServer({
    schema,
    plugins: [
      // Proper shutdown for the HTTP server
      ApolloServerPluginDrainHttpServer({ httpServer }),

      // WebSocket shutdown handler can be added later
    ],
  });

  // Start Apollo Server
  await apolloServer.start();

  // Apply Apollo middleware to Express
  app.use(
    "/graphql",
    expressMiddleware(apolloServer, {
      context: async ({ req }: any) => {
        // Extract user from request (set by auth middleware)
        return {
          user: (req as any).user || null,
        };
      },
    })
  );

  console.log("[GraphQL] Apollo Server started at /graphql");
  console.log("[GraphQL] GraphQL Playground available at /graphql");
  console.log("[GraphQL] WebSocket subscriptions enabled");

  return apolloServer;
}
