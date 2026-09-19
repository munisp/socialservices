import { useState, useEffect, useRef } from "react";
import { Search, X, Loader2, User, Settings, Database, FileText, Save, Bookmark } from "lucide-react";
import { trpc } from "@/lib/trpc";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { useLocation } from "wouter";

interface GlobalSearchProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const typeIcons = {
  admin_portal_programs: Settings,
  admin_portal_users: User,
  admin_portal_mcc_codes: Database,
  admin_portal_audit_logs: FileText,
};

const typeLabels = {
  admin_portal_programs: "Program",
  admin_portal_users: "User",
  admin_portal_mcc_codes: "MCC Code",
  admin_portal_audit_logs: "Audit Log",
};

const typeColors = {
  admin_portal_programs: "bg-blue-100 text-blue-800",
  admin_portal_users: "bg-green-100 text-green-800",
  admin_portal_mcc_codes: "bg-purple-100 text-purple-800",
  admin_portal_audit_logs: "bg-orange-100 text-orange-800",
};

export default function GlobalSearch({ open, onOpenChange }: GlobalSearchProps) {
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [showSaveDialog, setShowSaveDialog] = useState(false);
  const [filterName, setFilterName] = useState("");

  const { data: savedFilters } = trpc.savedFilters.list.useQuery();
  const createFilterMutation = trpc.savedFilters.create.useMutation({
    onSuccess: () => {
      toast.success("Filter saved successfully");
      setShowSaveDialog(false);
      setFilterName("");
    },
  });
  const deleteFilterMutation = trpc.savedFilters.delete.useMutation({
    onSuccess: () => {
      toast.success("Filter deleted successfully");
    },
  });
  const inputRef = useRef<HTMLInputElement>(null);
  const [, setLocation] = useLocation();

  const { data: results, isLoading } = trpc.search.global.useQuery(
    { query: debouncedQuery },
    { enabled: debouncedQuery.length >= 2 }
  );

  // Debounce search query
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(query);
    }, 300);

    return () => clearTimeout(timer);
  }, [query]);

  // Focus input when dialog opens
  useEffect(() => {
    if (open) {
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [open]);

  // Handle keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        onOpenChange(true);
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [onOpenChange]);

  const handleResultClick = (result: any) => {
    onOpenChange(false);
    setQuery("");

    // Navigate based on result type
    if (result.index === "admin_portal_programs") {
      setLocation(`/programs/${result.source.id}`);
    } else if (result.index === "admin_portal_users") {
      setLocation(`/users`);
    } else if (result.index === "admin_portal_mcc_codes") {
      setLocation(`/mcc-management`);
    } else if (result.index === "admin_portal_audit_logs") {
      setLocation(`/users`); // Audit logs are on the users page
    }
  };

  const getResultTitle = (result: any) => {
    if (result.index === "admin_portal_programs") {
      return result.source.name;
    } else if (result.index === "admin_portal_users") {
      return result.source.name || result.source.email;
    } else if (result.index === "admin_portal_mcc_codes") {
      return `${result.source.mccCode} - ${result.source.description}`;
    } else if (result.index === "admin_portal_audit_logs") {
      return `${result.source.action} by ${result.source.performedByName}`;
    }
    return "Unknown";
  };

  const getResultDescription = (result: any) => {
    if (result.index === "admin_portal_programs") {
      return result.source.description || `Status: ${result.source.status}`;
    } else if (result.index === "admin_portal_users") {
      return `Role: ${result.source.role} • ${result.source.email}`;
    } else if (result.index === "admin_portal_mcc_codes") {
      return result.source.category || "No category";
    } else if (result.index === "admin_portal_audit_logs") {
      return result.source.justification || new Date(result.source.timestamp).toLocaleString();
    }
    return "";
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Search</DialogTitle>
        </DialogHeader>

        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            ref={inputRef}
            placeholder="Search programs, users, MCC codes, audit logs..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="pl-9 pr-9"
          />
          {query && (
            <Button
              variant="ghost"
              size="sm"
              className="absolute right-1 top-1/2 -translate-y-1/2 h-7 w-7 p-0"
              onClick={() => setQuery("")}
            >
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>

        <div className="max-h-[400px] overflow-y-auto">
          {isLoading && debouncedQuery && (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          )}

          {!isLoading && debouncedQuery && results && results.length === 0 && (
            <div className="text-center py-8 text-muted-foreground">
              No results found for "{debouncedQuery}"
            </div>
          )}

          {!isLoading && results && results.length > 0 && (
            <div className="space-y-2">
              {results.map((result, index) => {
                const Icon = typeIcons[result.index as keyof typeof typeIcons] || Search;
                const label = typeLabels[result.index as keyof typeof typeLabels] || "Unknown";
                const colorClass = typeColors[result.index as keyof typeof typeColors] || "bg-gray-100 text-gray-800";

                return (
                  <button
                    key={`${result.index}-${result.id}-${index}`}
                    onClick={() => handleResultClick(result)}
                    className="w-full text-left p-3 rounded-lg hover:bg-accent transition-colors"
                  >
                    <div className="flex items-start gap-3">
                      <div className="mt-0.5">
                        <Icon className="h-5 w-5 text-muted-foreground" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="font-medium truncate">{getResultTitle(result)}</span>
                          <Badge variant="secondary" className={`${colorClass} text-xs`}>
                            {label}
                          </Badge>
                        </div>
                        <p className="text-sm text-muted-foreground truncate">
                          {getResultDescription(result)}
                        </p>
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          )}

          {!debouncedQuery && (
            <div className="text-center py-8 text-muted-foreground text-sm">
              <p>Type to search across all entities</p>
              <p className="mt-2 text-xs">
                <kbd className="px-2 py-1 bg-muted rounded">⌘K</kbd> or{" "}
                <kbd className="px-2 py-1 bg-muted rounded">Ctrl+K</kbd> to open
              </p>
            </div>
          )}
        </div>

        {/* Save Filter Button */}
        {query && (
          <div className="flex items-center gap-2 px-4 py-2 border-t">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setShowSaveDialog(true)}
              className="w-full"
            >
              <Save className="h-4 w-4 mr-2" />
              Save Current Search
            </Button>
          </div>
        )}

        {/* Saved Filters */}
        {savedFilters && savedFilters.length > 0 && (
          <div className="px-4 py-2 border-t">
            <p className="text-xs font-medium text-muted-foreground mb-2">Saved Filters</p>
            <div className="space-y-1">
              {savedFilters.map((filter) => (
                <div key={filter.id} className="flex items-center gap-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setQuery(filter.query);
                    }}
                    className="flex-1 justify-start text-xs"
                  >
                    <Bookmark className="h-3 w-3 mr-2" />
                    {filter.name}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => deleteFilterMutation.mutate({ id: filter.id })}
                    className="h-7 w-7 p-0"
                  >
                    <X className="h-3 w-3" />
                  </Button>
                </div>
              ))}
            </div>
          </div>
        )}
      </DialogContent>

      {/* Save Filter Dialog */}
      <Dialog open={showSaveDialog} onOpenChange={setShowSaveDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Save Search Filter</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">Filter Name</label>
              <Input
                value={filterName}
                onChange={(e) => setFilterName(e.target.value)}
                placeholder="e.g., Active Programs"
                className="mt-1"
              />
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setShowSaveDialog(false)}>
                Cancel
              </Button>
              <Button
                onClick={() => {
                  if (!filterName.trim()) {
                    toast.error("Please enter a filter name");
                    return;
                  }
                  createFilterMutation.mutate({
                    name: filterName,
                    query,
                    entityType: "all",
                  });
                }}
                disabled={createFilterMutation.isPending}
              >
                Save Filter
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </Dialog>
  );
}
