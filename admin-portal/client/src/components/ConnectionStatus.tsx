import { useWebSocket } from "@/hooks/useWebSocket";
import { Wifi, WifiOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export function ConnectionStatus() {
  const { isConnected, reconnectAttempts } = useWebSocket();

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge
          variant={isConnected ? "outline" : "destructive"}
          className="gap-1.5 cursor-pointer"
        >
          {isConnected ? (
            <>
              <Wifi className="h-3 w-3" />
              <span className="text-xs">Connected</span>
            </>
          ) : (
            <>
              <WifiOff className="h-3 w-3" />
              <span className="text-xs">
                {reconnectAttempts > 0 ? `Reconnecting (${reconnectAttempts})` : "Disconnected"}
              </span>
            </>
          )}
        </Badge>
      </TooltipTrigger>
      <TooltipContent>
        <p className="text-xs">
          {isConnected
            ? "Real-time updates active"
            : reconnectAttempts > 0
            ? `Attempting to reconnect... (attempt ${reconnectAttempts})`
            : "Real-time updates unavailable"}
        </p>
      </TooltipContent>
    </Tooltip>
  );
}
