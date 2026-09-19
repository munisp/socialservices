import { useMemo } from "react";
import { Gantt, Task, ViewMode } from "gantt-task-react";
import "gantt-task-react/dist/index.css";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export interface WorkflowActivity {
  id: string;
  name: string;
  status: "scheduled" | "started" | "completed" | "failed" | "timeout" | "cancelled";
  startTime: Date;
  endTime?: Date;
  duration?: number;
  dependencies?: string[];
  parallel?: boolean;
}

export interface WorkflowTimelineProps {
  workflowId: string;
  workflowType: string;
  activities: WorkflowActivity[];
  startTime: Date;
  endTime?: Date;
}

/**
 * Workflow Timeline Visualization Component
 * Displays workflow execution timeline with Gantt chart showing:
 * - Activity dependencies
 * - Parallel execution paths
 * - Critical path
 * - Activity durations and status
 */
export function WorkflowTimeline({ workflowId, workflowType, activities, startTime, endTime }: WorkflowTimelineProps) {
  // Transform workflow activities into Gantt tasks
  const tasks: Task[] = useMemo(() => {
    if (!activities || activities.length === 0) {
      return [];
    }

    return activities.map((activity, index) => {
      const start = new Date(activity.startTime);
      const end = activity.endTime
        ? new Date(activity.endTime)
        : activity.status === "completed" || activity.status === "failed"
        ? new Date(activity.startTime.getTime() + (activity.duration || 1000))
        : new Date(); // For running activities, use current time

      // Determine task type based on dependencies and parallel execution
      const type: Task["type"] = activity.dependencies && activity.dependencies.length > 0 ? "task" : "task";

      // Determine progress based on status
      const progress =
        activity.status === "completed"
          ? 100
          : activity.status === "failed"
          ? 100
          : activity.status === "started"
          ? 50
          : 0;

      // Determine color based on status
      const styles = {
        backgroundColor:
          activity.status === "completed"
            ? "#10b981" // green
            : activity.status === "failed"
            ? "#ef4444" // red
            : activity.status === "started"
            ? "#3b82f6" // blue
            : "#94a3b8", // gray
        backgroundSelectedColor:
          activity.status === "completed"
            ? "#059669"
            : activity.status === "failed"
            ? "#dc2626"
            : activity.status === "started"
            ? "#2563eb"
            : "#64748b",
      };

      return {
        id: activity.id,
        name: activity.name,
        start,
        end,
        progress,
        type,
        dependencies: activity.dependencies || [],
        styles,
        isDisabled: false,
      };
    });
  }, [activities]);

  // Calculate critical path (longest path through the workflow)
  const criticalPath = useMemo(() => {
    if (!activities || activities.length === 0) return [];

    // Simple critical path calculation: find activities with no dependencies or longest chain
    const pathLengths = new Map<string, number>();

    const calculatePathLength = (activityId: string, visited = new Set<string>()): number => {
      if (pathLengths.has(activityId)) {
        return pathLengths.get(activityId)!;
      }

      if (visited.has(activityId)) {
        return 0; // Circular dependency, return 0
      }

      visited.add(activityId);

      const activity = activities.find((a) => a.id === activityId);
      if (!activity) return 0;

      const activityDuration = activity.duration || 0;

      if (!activity.dependencies || activity.dependencies.length === 0) {
        pathLengths.set(activityId, activityDuration);
        return activityDuration;
      }

      const maxDependencyPath = Math.max(
        ...activity.dependencies.map((depId) => calculatePathLength(depId, new Set(visited)))
      );

      const totalPath = maxDependencyPath + activityDuration;
      pathLengths.set(activityId, totalPath);
      return totalPath;
    };

    // Calculate path lengths for all activities
    activities.forEach((activity) => {
      calculatePathLength(activity.id);
    });

    // Find the maximum path length
    const maxPathLength = Math.max(...Array.from(pathLengths.values()));

    // Return activities on the critical path
    return activities.filter((activity) => pathLengths.get(activity.id) === maxPathLength).map((a) => a.id);
  }, [activities]);

  // Calculate parallel execution groups
  const parallelGroups = useMemo(() => {
    if (!activities || activities.length === 0) return [];

    const groups: string[][] = [];
    const processed = new Set<string>();

    activities.forEach((activity) => {
      if (processed.has(activity.id)) return;

      if (activity.parallel) {
        // Find all activities that run in parallel with this one
        const parallelActivities = activities.filter(
          (a) =>
            a.parallel &&
            !processed.has(a.id) &&
            Math.abs(a.startTime.getTime() - activity.startTime.getTime()) < 1000 // Within 1 second
        );

        if (parallelActivities.length > 1) {
          const group = parallelActivities.map((a) => a.id);
          groups.push(group);
          group.forEach((id) => processed.add(id));
        }
      }
    });

    return groups;
  }, [activities]);

  if (tasks.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Workflow Timeline</CardTitle>
          <CardDescription>No activity data available</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Workflow Execution Timeline</CardTitle>
        <CardDescription>
          {workflowType} - {workflowId}
        </CardDescription>
        <div className="flex gap-2 mt-2">
          {criticalPath.length > 0 && (
            <Badge variant="outline">
              Critical Path: {criticalPath.length} activities
            </Badge>
          )}
          {parallelGroups.length > 0 && (
            <Badge variant="outline">
              {parallelGroups.length} parallel execution groups
            </Badge>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="overflow-x-auto">
          <Gantt
            tasks={tasks}
            viewMode={ViewMode.Hour}
            listCellWidth="200px"
            columnWidth={60}
            rowHeight={50}
            barCornerRadius={4}
            handleWidth={8}
            fontSize="14px"
            fontFamily="Inter, system-ui, sans-serif"
          />
        </div>

        {/* Legend */}
        <div className="mt-4 flex gap-4 text-sm">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: "#10b981" }}></div>
            <span>Completed</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: "#3b82f6" }}></div>
            <span>Running</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: "#ef4444" }}></div>
            <span>Failed</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 rounded" style={{ backgroundColor: "#94a3b8" }}></div>
            <span>Scheduled</span>
          </div>
        </div>

        {/* Critical Path Highlight */}
        {criticalPath.length > 0 && (
          <div className="mt-4 p-3 bg-orange-50 dark:bg-orange-950 rounded-lg border border-orange-200 dark:border-orange-800">
            <h4 className="font-semibold text-sm mb-2">Critical Path Activities:</h4>
            <div className="flex flex-wrap gap-2">
              {criticalPath.map((activityId) => {
                const activity = activities.find((a) => a.id === activityId);
                return activity ? (
                  <Badge key={activityId} variant="secondary">
                    {activity.name}
                  </Badge>
                ) : null;
              })}
            </div>
          </div>
        )}

        {/* Parallel Execution Groups */}
        {parallelGroups.length > 0 && (
          <div className="mt-4 p-3 bg-blue-50 dark:bg-blue-950 rounded-lg border border-blue-200 dark:border-blue-800">
            <h4 className="font-semibold text-sm mb-2">Parallel Execution Groups:</h4>
            {parallelGroups.map((group, index) => (
              <div key={index} className="mb-2">
                <span className="text-xs text-muted-foreground">Group {index + 1}:</span>
                <div className="flex flex-wrap gap-2 mt-1">
                  {group.map((activityId) => {
                    const activity = activities.find((a) => a.id === activityId);
                    return activity ? (
                      <Badge key={activityId} variant="outline">
                        {activity.name}
                      </Badge>
                    ) : null;
                  })}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
