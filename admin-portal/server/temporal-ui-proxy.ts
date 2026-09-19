import { Request, Response, NextFunction } from "express";
import { createProxyMiddleware, Options } from "http-proxy-middleware";

/**
 * Temporal UI Proxy Configuration
 * 
 * This module sets up a reverse proxy to the Temporal UI,
 * allowing embedded access within the Admin Portal.
 */

const TEMPORAL_UI_URL = process.env.TEMPORAL_UI_URL || "http://localhost:8080";

/**
 * Authentication middleware for Temporal UI access
 * Only admin users can access Temporal UI
 */
export function temporalUIAuthMiddleware(req: Request, res: Response, next: NextFunction) {
  // Check if user is authenticated and has admin role
  // This assumes session middleware has already run
  const user = (req as any).user;
  
  if (!user) {
    return res.status(401).json({ error: "Authentication required" });
  }
  
  if (user.role !== "admin") {
    return res.status(403).json({ error: "Admin access required" });
  }
  
  next();
}

/**
 * Create Temporal UI proxy middleware
 */
const proxyOptions: Options = {
  target: TEMPORAL_UI_URL,
  changeOrigin: true,
  pathRewrite: {
    "^/api/temporal-ui": "", // Remove /api/temporal-ui prefix
  },
};

export const temporalUIProxy = createProxyMiddleware(proxyOptions);

/**
 * Temporal UI health check endpoint
 */
export async function temporalUIHealthCheck(req: Request, res: Response) {
  try {
    const response = await fetch(`${TEMPORAL_UI_URL}/health`);
    
    if (response.ok) {
      res.json({
        status: "healthy",
        temporalUI: TEMPORAL_UI_URL,
      });
    } else {
      res.status(503).json({
        status: "unhealthy",
        temporalUI: TEMPORAL_UI_URL,
      });
    }
  } catch (error) {
    res.status(503).json({
      status: "unavailable",
      temporalUI: TEMPORAL_UI_URL,
      error: error instanceof Error ? error.message : "Unknown error",
    });
  }
}

/**
 * Get Temporal workflow URL for direct linking
 */
export function getTemporalWorkflowURL(workflowId: string, runId: string): string {
  const namespace = process.env.TEMPORAL_NAMESPACE || "default";
  return `${TEMPORAL_UI_URL}/namespaces/${namespace}/workflows/${workflowId}/${runId}`;
}
