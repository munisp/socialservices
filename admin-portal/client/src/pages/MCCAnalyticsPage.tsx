import DashboardLayout from "@/components/DashboardLayout";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { trpc } from "@/lib/trpc";
import { Settings, Flag, Calendar, Shield, Loader2, BarChart3, TrendingUp } from "lucide-react";

export default function MCCAnalyticsPage() {
  const { data: analytics, isLoading } = trpc.mccAnalytics.getUsageStats.useQuery();

  return (
    <DashboardLayout
      items={[
        { label: "Dashboard", href: "/", icon: Shield },
        { label: "Programs & MCC Rules", href: "/programs", icon: Settings },
        { label: "Feature Flags", href: "/feature-flags", icon: Flag },
        { label: "Disbursements", href: "/disbursements", icon: Calendar },
      ]}
    >
      <div className="space-y-6">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">MCC Usage Analytics</h1>
          <p className="text-muted-foreground mt-2">
            Analyze Merchant Category Code usage patterns across benefit programs
          </p>
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : !analytics ? (
          <div className="text-center py-12 text-muted-foreground">
            No analytics data available
          </div>
        ) : (
          <>
            {/* Summary Cards */}
            <div className="grid gap-4 md:grid-cols-3">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total MCC Codes</CardTitle>
                  <BarChart3 className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{analytics.totalMccCodes}</div>
                  <p className="text-xs text-muted-foreground">
                    Unique codes across all programs
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total MCC Rules</CardTitle>
                  <TrendingUp className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{analytics.totalRules}</div>
                  <p className="text-xs text-muted-foreground">
                    Total rules across {analytics.totalPrograms} programs
                  </p>
                </CardContent>
              </Card>
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Avg Rules per Program</CardTitle>
                  <Settings className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {analytics.totalPrograms > 0
                      ? (analytics.totalRules / analytics.totalPrograms).toFixed(1)
                      : "0"}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Average MCC rules per program
                  </p>
                </CardContent>
              </Card>
            </div>

            {/* Top MCC Codes Table */}
            <Card>
              <CardHeader>
                <CardTitle>Top 20 Most Used MCC Codes</CardTitle>
                <CardDescription>
                  Merchant Category Codes ranked by frequency across all benefit programs
                </CardDescription>
              </CardHeader>
              <CardContent>
                {analytics.topMccCodes.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    No MCC rules configured yet
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-12">#</TableHead>
                        <TableHead>MCC Code</TableHead>
                        <TableHead>Description</TableHead>
                        <TableHead className="text-center">Usage Count</TableHead>
                        <TableHead>Programs</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {analytics.topMccCodes.map((mcc, index) => (
                        <TableRow key={mcc.code}>
                          <TableCell className="font-medium text-muted-foreground">
                            {index + 1}
                          </TableCell>
                          <TableCell>
                            <code className="bg-muted px-2 py-1 rounded text-sm font-mono">
                              {mcc.code}
                            </code>
                          </TableCell>
                          <TableCell className="max-w-md">{mcc.description}</TableCell>
                          <TableCell className="text-center">
                            <span className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-blue-100 text-blue-800 font-semibold text-sm">
                              {mcc.count}
                            </span>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-1">
                              {mcc.programs.slice(0, 3).map((program, idx) => (
                                <span
                                  key={idx}
                                  className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800"
                                >
                                  {program}
                                </span>
                              ))}
                              {mcc.programs.length > 3 && (
                                <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">
                                  +{mcc.programs.length - 3} more
                                </span>
                              )}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            {/* Visual Bar Chart */}
            <Card>
              <CardHeader>
                <CardTitle>MCC Usage Distribution</CardTitle>
                <CardDescription>
                  Visual representation of top 10 MCC codes by usage frequency
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {analytics.topMccCodes.slice(0, 10).map((mcc) => {
                    const maxCount = analytics.topMccCodes[0]?.count || 1;
                    const widthPercent = (mcc.count / maxCount) * 100;
                    return (
                      <div key={mcc.code} className="space-y-1">
                        <div className="flex items-center justify-between text-sm">
                          <span className="font-mono font-medium">{mcc.code}</span>
                          <span className="text-muted-foreground">{mcc.count} programs</span>
                        </div>
                        <div className="w-full bg-gray-200 rounded-full h-2.5">
                          <div
                            className="bg-blue-600 h-2.5 rounded-full transition-all duration-500"
                            style={{ width: `${widthPercent}%` }}
                          ></div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </CardContent>
            </Card>
          </>
        )}
      </div>
    </DashboardLayout>
  );
}
