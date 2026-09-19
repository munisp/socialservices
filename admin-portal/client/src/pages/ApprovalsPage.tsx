import DashboardLayout from "@/components/DashboardLayout";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { trpc } from "@/lib/trpc";
import { CheckCircle2, XCircle, Clock, AlertCircle } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

export default function ApprovalsPage() {
  const { data: requests, isLoading, refetch } = trpc.approvals.getPending.useQuery();
  const approveMutation = trpc.approvals.approve.useMutation();
  
  const [selectedRequest, setSelectedRequest] = useState<any>(null);
  const [showApprovalDialog, setShowApprovalDialog] = useState(false);
  const [decision, setDecision] = useState<"approved" | "rejected">("approved");
  const [comment, setComment] = useState("");

  const handleApprove = async () => {
    if (!selectedRequest) return;

    try {
      await approveMutation.mutateAsync({
        requestId: selectedRequest.id,
        decision,
        comment: comment || undefined,
      });

      toast.success(decision === "approved" ? "Request approved" : "Request rejected");
      setShowApprovalDialog(false);
      setComment("");
      refetch();
    } catch (error: any) {
      toast.error(error.message || "Failed to process approval");
    }
  };

  const getRequestTypeLabel = (type: string) => {
    const labels: Record<string, string> = {
      ROLE_CHANGE: "Role Change",
      BATCH_PROGRAM_UPDATE: "Batch Program Update",
      BATCH_FLAG_TOGGLE: "Batch Flag Toggle",
      PROGRAM_DELETE: "Program Deletion",
      USER_DELETE: "User Deletion",
    };
    return labels[type] || type;
  };

  const getRequestTypeBadge = (type: string) => {
    const variants: Record<string, any> = {
      ROLE_CHANGE: "default",
      BATCH_PROGRAM_UPDATE: "secondary",
      BATCH_FLAG_TOGGLE: "outline",
      PROGRAM_DELETE: "destructive",
      USER_DELETE: "destructive",
    };
    return variants[type] || "default";
  };

  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold">Pending Approvals</h1>
          <p className="text-muted-foreground">
            Review and approve critical operations requiring multi-level authorization
          </p>
        </div>

        {isLoading ? (
          <div className="space-y-4">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-32 bg-muted animate-pulse rounded-lg" />
            ))}
          </div>
        ) : requests && requests.length > 0 ? (
          <div className="space-y-4">
            {requests.map((request: any) => (
              <Card key={request.id}>
                <CardHeader>
                  <div className="flex items-start justify-between">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <CardTitle className="text-lg">{getRequestTypeLabel(request.requestType)}</CardTitle>
                        <Badge variant={getRequestTypeBadge(request.requestType)}>
                          {request.requestType}
                        </Badge>
                      </div>
                      <CardDescription>
                        Requested by User #{request.requestedBy} on{" "}
                        {new Date(request.createdAt).toLocaleDateString()}
                      </CardDescription>
                    </div>
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Clock className="h-4 w-4" />
                      <span>
                        {request.currentApprovals} / {request.requiredApprovals} approvals
                      </span>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <h4 className="text-sm font-medium mb-1">Justification</h4>
                    <p className="text-sm text-muted-foreground">{request.justification}</p>
                  </div>

                  <div>
                    <h4 className="text-sm font-medium mb-1">Request Details</h4>
                    <pre className="text-xs bg-muted p-3 rounded overflow-auto">
                      {JSON.stringify(request.requestData, null, 2)}
                    </pre>
                  </div>

                  <div className="flex gap-2 pt-2">
                    <Button
                      size="sm"
                      onClick={() => {
                        setSelectedRequest(request);
                        setDecision("approved");
                        setShowApprovalDialog(true);
                      }}
                    >
                      <CheckCircle2 className="h-4 w-4 mr-2" />
                      Approve
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      onClick={() => {
                        setSelectedRequest(request);
                        setDecision("rejected");
                        setShowApprovalDialog(true);
                      }}
                    >
                      <XCircle className="h-4 w-4 mr-2" />
                      Reject
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        ) : (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <AlertCircle className="h-12 w-12 text-muted-foreground mb-4" />
              <h3 className="text-lg font-medium mb-1">No Pending Approvals</h3>
              <p className="text-sm text-muted-foreground">
                All approval requests have been processed
              </p>
            </CardContent>
          </Card>
        )}
      </div>

      <Dialog open={showApprovalDialog} onOpenChange={setShowApprovalDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {decision === "approved" ? "Approve Request" : "Reject Request"}
            </DialogTitle>
            <DialogDescription>
              {decision === "approved"
                ? "This request will be executed once it reaches the required number of approvals."
                : "This request will be permanently rejected and cannot be undone."}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-4">
            <div>
              <label className="text-sm font-medium">Comment (Optional)</label>
              <Textarea
                placeholder="Add a comment explaining your decision..."
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                rows={3}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setShowApprovalDialog(false)}>
              Cancel
            </Button>
            <Button
              variant={decision === "approved" ? "default" : "destructive"}
              onClick={handleApprove}
              disabled={approveMutation.isPending}
            >
              {approveMutation.isPending ? "Processing..." : decision === "approved" ? "Approve" : "Reject"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DashboardLayout>
  );
}
