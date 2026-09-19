import DashboardLayout from "@/components/DashboardLayout";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Link } from "wouter";
import { Settings, Flag, Calendar, Shield, Users, Upload, TrendingUp, UserCheck, Activity, Database, Clock, BarChart3, FileText, Copy, CheckSquare, CreditCard, AlertTriangle, Download, Server } from "lucide-react";
import { trpc } from "@/lib/trpc";
import { useWebSocket } from "@/hooks/useWebSocket";
import { useEffect } from "react";

export default function Home() {
  const { data: analytics, isLoading: analyticsLoading, refetch: refetchAnalytics } = trpc.analytics.getDashboardMetrics.useQuery();
  const { socket, isConnected } = useWebSocket();
  
  // Listen for real-time dashboard updates
  useEffect(() => {
    if (!socket) return;
    
    const handleDashboardUpdate = () => {
      refetchAnalytics();
    };
    
    const handleProgramUpdate = () => {
      refetchAnalytics();
    };
    
    const handleUserUpdate = () => {
      refetchAnalytics();
    };
    
    const handleFeatureFlagUpdate = () => {
      refetchAnalytics();
    };
    
    socket.on("dashboard:update", handleDashboardUpdate);
    socket.on("program:update", handleProgramUpdate);
    socket.on("user:update", handleUserUpdate);
    socket.on("feature_flag:update", handleFeatureFlagUpdate);
    
    return () => {
      socket.off("dashboard:update", handleDashboardUpdate);
      socket.off("program:update", handleProgramUpdate);
      socket.off("user:update", handleUserUpdate);
      socket.off("feature_flag:update", handleFeatureFlagUpdate);
    };
  }, [socket, refetchAnalytics]);
  
  const features = [
    {
      title: "Benefit Programs & MCC Rules",
      description: "Manage social benefit programs and control earmarked spending through Merchant Category Code rules.",
      icon: Settings,
      href: "/programs",
      color: "text-blue-600",
    },
    {
      title: "Feature Flags",
      description: "Control platform features across different environments with centralized feature flag management.",
      icon: Flag,
      href: "/feature-flags",
      color: "text-green-600",
    },
    {
      title: "Disbursement Schedules",
      description: "Plan and track social benefit payment schedules for all programs.",
      icon: Calendar,
      href: "/disbursements",
      color: "text-purple-600",
    },
    {
      title: "User Management",
      description: "Manage user roles and view audit logs of all administrative actions.",
      icon: Users,
      href: "/users",
      color: "text-orange-600",
    },
    {
      title: "Bulk MCC Import",
      description: "Upload CSV files to import Merchant Category Codes in bulk.",
      icon: Upload,
      href: "/mcc-import",
      color: "text-indigo-600",
    },
    {
      title: "MCC Database Management",
      description: "Search, edit, and manage individual Merchant Category Code entries.",
      icon: Database,
      href: "/mcc-management",
      color: "text-cyan-600",
    },
    {
      title: "MCC Usage Analytics",
      description: "Analyze Merchant Category Code usage patterns across all benefit programs.",
      icon: BarChart3,
      href: "/mcc-analytics",
      color: "text-pink-600",
    },
    {
      title: "Scheduled Reports",
      description: "Configure automated weekly and monthly summary reports delivered via email.",
      icon: FileText,
      href: "/reports",
      color: "text-indigo-600",
    },
    {
      title: "Program Templates",
      description: "Manage saved program templates for quick duplication with pre-configured MCC rules.",
      icon: Copy,
      href: "/templates",
      color: "text-purple-600",
    },
    {
      title: "Notification Settings",
      description: "Configure email notification preferences for admin actions and events.",
      icon: Settings,
      href: "/settings",
      color: "text-gray-600",
    },
    {
      title: "Pending Approvals",
      description: "Review and approve critical operations requiring multi-level authorization.",
      icon: CheckSquare,
      href: "/approvals",
      color: "text-green-600",
    },
    {
      title: "Beneficiary Management",
      description: "Manage beneficiary enrollments, KYC verification, and benefit card issuance.",
      icon: UserCheck,
      href: "/beneficiaries",
      color: "text-blue-600",
    },
    {
      title: "Transaction Monitoring",
      description: "Real-time transaction monitoring, compliance checking, and fraud detection.",
      icon: AlertTriangle,
      href: "/transactions",
      color: "text-red-600",
    },
    {
      title: "Data Export & Reporting",
      description: "Export platform data in CSV or Excel format for analysis and compliance.",
      icon: Download,
      href: "/data-export",
      color: "text-teal-600",
    },
    {
      title: "Middleware Dashboard",
      description: "Unified monitoring for all 8 middleware components with real-time health status, metrics, and alerts.",
      icon: Server,
      href: "/middleware",
      color: "text-purple-600",
    },
  ];

  return (
    <DashboardLayout
      items={[
        { label: "Dashboard", href: "/", icon: Shield },
        { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
        { label: "Feature Flags", href: "/feature-flags", icon: Flag },
        { label: "Disbursements", href: "/disbursements", icon: Calendar },
        { label: "User Management", href: "/users", icon: Users },
      ]}
    >
        <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Admin Portal</h1>
          <p className="text-muted-foreground mt-2">Centralized control for the Social Protection Platform</p>
        </div>

        {/* Analytics Dashboard */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Total Users</CardTitle>
              <Users className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{analyticsLoading ? "—" : analytics?.totalUsers || 0}</div>
              <p className="text-xs text-muted-foreground">
                {analyticsLoading ? "—" : `${analytics?.adminCount || 0} admins`}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Benefit Programs</CardTitle>
              <Settings className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{analyticsLoading ? "—" : analytics?.totalPrograms || 0}</div>
              <p className="text-xs text-muted-foreground">Active programs</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Feature Flags</CardTitle>
              <Flag className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{analyticsLoading ? "—" : analytics?.totalFlags || 0}</div>
              <p className="text-xs text-muted-foreground">Configured flags</p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">Upcoming Disbursements</CardTitle>
              <Calendar className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent>
              <div className="text-2xl font-bold">{analyticsLoading ? "—" : analytics?.upcomingDisbursements || 0}</div>
              <p className="text-xs text-muted-foreground">Pending payments</p>
            </CardContent>
          </Card>
        </div>

        {/* Recent Activity Timeline */}
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2">
              <Clock className="h-5 w-5" />
              <CardTitle>Recent Activity</CardTitle>
            </div>
            <CardDescription>Last 10 administrative actions</CardDescription>
          </CardHeader>
          <CardContent>
            {analyticsLoading ? (
              <div className="space-y-3">
                {[...Array(5)].map((_, i) => (
                  <div key={i} className="h-12 bg-muted animate-pulse rounded" />
                ))}
              </div>
            ) : analytics?.recentActions && analytics.recentActions.length > 0 ? (
              <div className="space-y-3">
                {analytics.recentActions.map((action: any) => (
                  <div key={action.id} className="flex items-start gap-3 pb-3 border-b last:border-0">
                    <div className="p-2 bg-muted rounded-full mt-1">
                      <Activity className="h-3 w-3" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium truncate">{action.action.replace(/_/g, " ").toUpperCase()}</p>
                      <p className="text-xs text-muted-foreground">by User #{action.performedBy}</p>
                      <p className="text-xs text-muted-foreground">
                        {new Date(action.performedAt).toLocaleString()}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground text-center py-4">No recent activity</p>
            )}
          </CardContent>
        </Card>

        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {features.map((feature) => {
            const Icon = feature.icon;
            return (
              <Link key={feature.href} href={feature.href}>
                <Card className="hover:shadow-lg transition-shadow cursor-pointer h-full">
                  <CardHeader>
                    <div className="flex items-center gap-3">
                      <div className={`p-2 rounded-lg bg-muted ${feature.color}`}>
                        <Icon className="h-6 w-6" />
                      </div>
                      <CardTitle className="text-xl">{feature.title}</CardTitle>
                    </div>
                  </CardHeader>
                  <CardContent>
                    <CardDescription className="text-base">{feature.description}</CardDescription>
                  </CardContent>
                </Card>
              </Link>
            );
          })}
        </div>
      </div>
    </DashboardLayout>
  );
}
