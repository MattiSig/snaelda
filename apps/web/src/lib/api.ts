export type APIErrorPayload = {
  error?:
    | string
    | {
        code?: string;
        message?: string;
      };
  code?: string;
  message?: string;
};

export class APIError extends Error {
  readonly status: number;
  readonly payload: APIErrorPayload | null;

  constructor(status: number, payload: APIErrorPayload | null) {
    const nestedMessage =
      typeof payload?.error === "object" ? payload.error.message : undefined;
    super(
      payload?.message ??
        nestedMessage ??
        `API request failed with status ${status}`,
    );
    this.name = "APIError";
    this.status = status;
    this.payload = payload;
  }
}

export type AuthUser = {
  id: string;
  email: string;
  name: string;
  workspaceId: string;
  workspaceRole: string;
};

export type BuilderSession = {
  kind: "authenticated" | "trial";
  workspaceId: string;
  workspaceRole: string;
  user?: AuthUser;
  guestSessionId?: string;
  promptsUsed?: number;
  promptLimit?: number;
  trialStartedAt?: string;
  trialExpiresAt?: string;
  trialExpired?: boolean;
  claimedAt?: string;
  claimedByUserId?: string;
  hasRecoveryKey?: boolean;
  subscriptionLive?: boolean;
};

export type AuthSession = {
  user: AuthUser;
  expiresAt?: number;
  tokenType?: string;
};

export type SiteSummary = {
  id: string;
  workspaceId: string;
  name: string;
  slug: string;
  status: string;
  defaultLocale: string;
  publishedVersionId?: string;
  pageCount: number;
};

export type BrandConfig = {
  businessName: string;
  logo?: {
    assetId: string;
    alt: string;
  };
  primaryColor: string;
};

export type FooterContact = {
  address?: string;
  phone?: string;
  email?: string;
  hours?: string[];
};

export type SiteDraft = {
  site: {
    id: string;
    name: string;
    slug: string;
    status: string;
    defaultLocale?: string;
    seo?: {
      title?: string;
      description?: string;
    };
  };
  brand: BrandConfig;
  theme: {
    version: string;
    tokens: {
      colors: Record<string, string>;
      typography: Record<string, string | number>;
      layout: Record<string, string | number>;
      shape: Record<string, string | number>;
    };
  };
  navigation: {
    primary: Array<{
      label: string;
      pageId?: string;
      href?: string;
    }>;
    footer?: Array<{
      label: string;
      pageId?: string;
      href?: string;
    }>;
  };
  pages: Array<{
    id: string;
    title: string;
    slug: string;
    status?: "draft" | "published";
    type?: PageType;
    collectionId?: string;
    seo?: {
      title?: string;
      description?: string;
    };
    settings?: Record<string, unknown>;
    blocks: Array<{
      id: string;
      type: string;
      version: string;
      props: Record<string, unknown>;
      settings?: {
        hidden?: boolean;
        anchorId?: string;
      };
      bindings?: Record<string, BlockBinding>;
    }>;
  }>;
  collections?: Collection[];
};

export type PageType = "static" | "collection_index" | "collection_detail";

export type BlockBinding = {
  source: "entry";
  field: string;
};

export type CollectionFieldType =
  | "text"
  | "long_text"
  | "rich_text"
  | "number"
  | "boolean"
  | "date"
  | "url"
  | "email"
  | "phone"
  | "location"
  | "enum"
  | "enum_multi"
  | "asset"
  | "asset_list"
  | "reference";

export type FieldDefinition = {
  key: string;
  label: string;
  type: CollectionFieldType;
  required?: boolean;
  description?: string;
  options?: string[];
  defaultValue?: unknown;
  validation?: {
    minLength?: number;
    maxLength?: number;
    min?: number;
    max?: number;
  };
};

export type CollectionSettings = {
  defaultSort?: string;
  exposeDetailUrls?: boolean;
};

export type CollectionEntry = {
  id: string;
  slug: string;
  fields: Record<string, unknown>;
  seo?: {
    title?: string;
    description?: string;
  };
  status?: "draft" | "published";
  sortOrder: number;
};

export type Collection = {
  id: string;
  slug: string;
  singularLabel: string;
  pluralLabel: string;
  schema: FieldDefinition[];
  settings?: CollectionSettings;
  sortOrder: number;
  entries?: CollectionEntry[];
};

export type ImageCredit = {
  provider: string;
  author?: string;
  authorUrl?: string;
  sourceUrl?: string;
  license?: string;
};

export type PublishedSnapshot = {
  schemaVersion: string;
  site: {
    id: string;
    name: string;
    defaultLocale: string;
    seo?: {
      title?: string;
      description?: string;
    };
  };
  brand: BrandConfig;
  theme: SiteDraft["theme"];
  navigation: SiteDraft["navigation"];
  pages: SiteDraft["pages"];
  collections?: Collection[];
  imageCredits?: ImageCredit[];
};

export type SiteVersion = {
  id: string;
  siteId: string;
  versionNumber: number;
  createdAt: string;
  publishNote?: string;
  isCurrent: boolean;
};

export type BlockEditorField = {
  name: string;
  label: string;
  control: string;
  valueType?: string;
  description?: string;
  placeholder?: string;
  options?: string[];
  fields?: BlockEditorField[];
  itemFields?: BlockEditorField[];
};

export type BlockDefinition = {
  type: string;
  version: string;
  displayName: string;
  category: string;
  defaultProps?: Record<string, unknown>;
  editorSchema?: BlockEditorField[];
};

export type GenerationMetadata = {
  prompt: string;
  themePreset?: string;
  assetsNeeded?: string[];
  assumptions?: string[];
  validationRetryCount?: number;
};

export type SiteDraftResponse = {
  draft: SiteDraft;
  generation: GenerationMetadata;
  blockRegistry: BlockDefinition[];
};

export type SiteRepromptResponse = {
  jobId: string;
  draft: SiteDraft;
};

export type DraftRevisionRecord = {
  id: string;
  scope: 'site' | 'page';
  targetId?: string;
  prompt: string;
  draft: SiteDraft;
  createdAt: string;
};

export type RepromptHistoryRecord = {
  id: string;
  scope: 'site' | 'page';
  targetId?: string;
  prompt: string;
  changeSummary?: string;
  previousRevisionId: string;
  resultRevisionId: string;
  jobId?: string;
  createdAt: string;
  undoneAt?: string;
};

export type GenerationProgressStep = {
  step: string;
  label: string;
  index: number;
  total: number;
};

export type GenerationJob = {
  id: string;
  kind: string;
  state: "pending" | "running" | "succeeded" | "failed" | "canceled";
  currentStep?: GenerationProgressStep;
  siteId?: string;
  errorReason?: string;
  startedAt?: string;
  completedAt?: string;
};

export type GenerationStreamResult = {
  jobId: string;
  siteId: string;
  draftId: string;
};

export type PublishSiteResponse = {
  version: SiteVersion;
  hostname: string;
  publicUrl: string;
  snapshot: PublishedSnapshot;
};

export type RollbackSiteResponse = {
  version: SiteVersion;
  hostname: string;
  publicUrl: string;
};

export type ThemeOption = {
  id: string;
  label: string;
  description?: string;
};

export type ThemeSelection = {
  palette: string;
  fontPreset: string;
  sectionSpacing: string;
  radius: string;
  buttonStyle: string;
  imageStyle: string;
};

export type ThemeEditorCatalog = {
  palettes: ThemeOption[];
  fontPresets: ThemeOption[];
  sectionSpacings: ThemeOption[];
  radii: ThemeOption[];
  buttonStyles: ThemeOption[];
  imageStyles: ThemeOption[];
};

export type ThemeState = {
  theme: SiteDraft["theme"];
  selection: ThemeSelection;
  options: ThemeEditorCatalog;
};

export type AssetProvenance = {
  provider: string;
  providerId?: string;
  author?: string;
  authorUrl?: string;
  license?: string;
  query?: string;
  sourceUrl?: string;
};

export type AssetMetadata = {
  fileName?: string;
  contentType?: string;
  requestedSizeBytes?: number;
  sizeBytes?: number;
  width?: number;
  height?: number;
  etag?: string;
  uploadStatus?: string;
  uploadedAt?: string;
  provenance?: AssetProvenance;
};

export type AssetRecord = {
  id: string;
  workspaceId: string;
  siteId?: string;
  kind: string;
  storageKey: string;
  publicUrl?: string;
  downloadUrl?: string;
  altText?: string;
  metadata: AssetMetadata;
  createdBy?: string;
  createdAt: string;
};

export type FormSubmissionStatus = "new" | "reviewed" | "resolved" | "spam";

export type FormSubmissionRecord = {
  id: string;
  siteId: string;
  pageId?: string;
  blockId?: string;
  status: FormSubmissionStatus;
  payload: Record<string, unknown>;
  createdAt: string;
  pageTitle?: string;
};

export type AssetUploadTicket = {
  asset: AssetRecord;
  upload: {
    url: string;
    method: string;
    headers?: Record<string, string>;
    expiresAt: string;
  };
};

export type SiteVersionsResponse = {
  versions: SiteVersion[];
};

export type SiteDomain = {
  id: string;
  hostname: string;
  type: string;
  status: string;
  publicUrl?: string;
  verificationHostname?: string;
  verificationValue?: string;
};

export type SiteDomainsResponse = {
  siteId: string;
  siteSlug: string;
  published: boolean;
  hostedHostname: string;
  publicUrl?: string;
  customDomainsEnabled: boolean;
  domains: SiteDomain[];
};

export type BillingEntitlement = {
  workspaceId: string;
  plan: string;
  status: string;
  subscriptionLive: boolean;
  customDomainsEnabled: boolean;
  activeSiteLimit?: number;
  monthlyPromptLimit?: number;
  assetStorageLimitBytes?: number;
  updatedAt: string;
};

export type BillingUsage = {
  activeSiteCount: number;
  periodPromptCount: number;
  uploadedAssetBytes: number;
  currentPeriodStart?: string;
  currentPeriodEnd?: string;
};

export type OnceOverStatus =
  | "none"
  | "awaiting_intake"
  | "pending"
  | "delivered";

export type OnceOverRequest = {
  id: string;
  paidAt: string;
  intakeBusiness?: string;
  intakeVisitor?: string;
  intakeOutcome?: string;
  intakeStuckOn?: string;
  intakeSubmittedAt?: string;
  videoUrl?: string;
  deliveredAt?: string;
};

export type OnceOverState = {
  status: OnceOverStatus;
  request?: OnceOverRequest;
};

export type BillingState = {
  entitlement: BillingEntitlement;
  usage: BillingUsage;
  onceOver: OnceOverState;
};

export type PublishedSiteResponse = {
  siteSlug: string;
  hostname?: string;
  brand?: BrandConfig;
  publicUrl: string;
  version: SiteVersion;
  pagePath: string;
  page: {
    pagePath: string;
    title: string;
    description: string;
    canonicalUrl: string;
    ogImageUrl?: string;
    localBusinessJsonLd?: Record<string, unknown>;
    html: string;
  };
};

export type PreviewTokenResponse = {
  token: string;
  expiresAt: string;
};

export type PreviewDraftResponse = {
  draft: SiteDraft;
};

export type SessionResponse = {
  session: BuilderSession;
};

const defaultAPIBaseURL = "http://localhost:8080";

export function getAPIBaseURL() {
  try {
    const value = (
      import.meta as ImportMeta & { env: { VITE_API_BASE_URL?: string } }
    ).env.VITE_API_BASE_URL;
    if (value) return value;
  } catch {
    // tsx/Node artifact renderer: `import.meta.env` is undefined; fall through.
  }
  if (typeof process !== "undefined" && process.env?.VITE_API_BASE_URL) {
    return process.env.VITE_API_BASE_URL;
  }
  return defaultAPIBaseURL;
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
  retryOnUnauthorized = true,
): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const response = await fetch(new URL(path, getAPIBaseURL()), {
    ...init,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...buildCSRFHeaders(method),
      ...init.headers,
    },
  });

  if (!response.ok) {
    if (
      response.status === 401 &&
      retryOnUnauthorized &&
      path !== "/api/auth/login" &&
      path !== "/api/auth/magic-link" &&
      path !== "/api/auth/refresh"
    ) {
      await refreshAuthSession();
      return apiFetch<T>(path, init, false);
    }

    const payload = await response.json().catch(() => null);
    throw new APIError(response.status, payload);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

function buildCSRFHeaders(method: string): HeadersInit {
  if (
    method === "GET" ||
    method === "HEAD" ||
    method === "OPTIONS" ||
    typeof document === "undefined"
  ) {
    return {};
  }

  const token = readCookie("snaelda_csrf_token");
  if (!token) {
    return {};
  }

  return {
    "X-CSRF-Token": token,
  };
}

function readCookie(name: string) {
  const prefix = `${name}=`;
  for (const cookie of document.cookie.split(";")) {
    const trimmed = cookie.trim();
    if (trimmed.startsWith(prefix)) {
      return decodeURIComponent(trimmed.slice(prefix.length));
    }
  }
  return "";
}

async function publicAPIRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetch(new URL(path, getAPIBaseURL()), {
    credentials: "omit",
    headers: {
      Accept: "application/json",
      ...init.headers,
    },
    ...init,
  });

  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new APIError(response.status, payload);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function streamAPIRequest(
  path: string,
  init: RequestInit,
  {
    onJobCreated,
    onProgress,
  }: {
    onJobCreated?: (jobId: string) => void;
    onProgress?: (step: GenerationProgressStep) => void;
  } = {},
  retryOnUnauthorized = true,
): Promise<GenerationStreamResult> {
  const method = (init.method ?? "GET").toUpperCase();
  const response = await fetch(new URL(path, getAPIBaseURL()), {
    ...init,
    credentials: "include",
    headers: {
      Accept: "text/event-stream",
      ...buildCSRFHeaders(method),
      ...init.headers,
    },
  });

  if (!response.ok) {
    if (
      response.status === 401 &&
      retryOnUnauthorized &&
      path !== "/api/auth/login" &&
      path !== "/api/auth/magic-link" &&
      path !== "/api/auth/refresh"
    ) {
      await refreshAuthSession();
      return streamAPIRequest(path, init, { onJobCreated, onProgress }, false);
    }

    const payload = await response.json().catch(() => null);
    throw new APIError(response.status, payload);
  }

  if (!response.body) {
    throw new APIError(500, {
      error: {
        code: "generation_stream_missing",
        message: "Generation progress stream was unavailable.",
      },
    });
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let jobId = "";
  let completed: GenerationStreamResult | null = null;

  async function handleEventBlock(block: string) {
    const lines = block
      .split(/\r?\n/)
      .map((line) => line.trimEnd())
      .filter(Boolean);
    let eventName = "message";
    const dataLines: string[] = [];
    for (const line of lines) {
      if (line.startsWith(":")) {
        continue;
      }
      if (line.startsWith("event:")) {
        eventName = line.slice("event:".length).trim();
        continue;
      }
      if (line.startsWith("data:")) {
        dataLines.push(line.slice("data:".length).trim());
      }
    }
    if (dataLines.length === 0) {
      return;
    }
    const payload = JSON.parse(dataLines.join("\n")) as Record<string, unknown>;
    if (eventName === "job") {
      jobId = String(payload.jobId ?? "");
      if (jobId && onJobCreated) {
        onJobCreated(jobId);
      }
      return;
    }
    if (eventName === "progress") {
      onProgress?.(payload as unknown as GenerationProgressStep);
      return;
    }
    if (eventName === "complete") {
      completed = {
        jobId: String(payload.jobId ?? jobId),
        siteId: String(payload.siteId ?? ""),
        draftId: String(payload.draftId ?? payload.siteId ?? ""),
      };
      return;
    }
    if (eventName === "failed") {
      const status = Number(payload.status ?? 500);
      throw new APIError(Number.isFinite(status) ? status : 500, {
        error: {
          code: String(payload.reason ?? "generation_failed"),
          message: String(
            payload.message ?? "We could not finish. Please try again.",
          ),
        },
      });
    }
  }

  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const blocks = buffer.split(/\r?\n\r?\n/);
    buffer = blocks.pop() ?? "";
    for (const block of blocks) {
      await handleEventBlock(block);
      if (completed) {
        return completed;
      }
    }
  }

  if (buffer.trim()) {
    await handleEventBlock(buffer);
  }
  if (completed) {
    return completed;
  }
  if (!jobId) {
    throw new APIError(500, {
      error: {
        code: "generation_stream_interrupted",
        message: "The generation connection dropped before progress could resume.",
      },
    });
  }

  for (let attempt = 0; attempt < 60; attempt += 1) {
    const { job } = await getGenerationJob(jobId);
    if (job.currentStep) {
      onProgress?.(job.currentStep);
    }
    if (job.state === "succeeded") {
      return {
        jobId,
        siteId: job.siteId ?? "",
        draftId: job.siteId ?? "",
      };
    }
    if (job.state === "failed" || job.state === "canceled") {
      throw new APIError(500, {
        error: {
          code: job.errorReason || "generation_failed",
          message: "We could not finish. Please try again.",
        },
      });
    }
    await sleep(2000);
  }

  throw new APIError(500, {
    error: {
      code: "generation_poll_timeout",
      message: "Generation took too long to confirm. Please try again.",
    },
  });
}

export async function getCurrentSession() {
  return apiFetch<SessionResponse>("/api/sessions/me").then(
    (response) => response.session,
  );
}

export async function getBillingState() {
  return apiFetch<BillingState>("/api/billing/entitlements");
}

export async function createBillingCheckout(input: {
  plan?: "basic" | "pro";
  purchaseType?: "subscription" | "once_over";
}) {
  return apiFetch<{ url: string }>("/api/billing/checkout", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function createBillingPortal() {
  return apiFetch<{ url: string }>("/api/billing/portal", {
    method: "POST",
  });
}

export async function updateOnceOver(input: {
  intakeBusiness: string;
  intakeVisitor: string;
  intakeOutcome: string;
  intakeStuckOn: string;
  readyForReview?: boolean;
}) {
  return apiFetch<{ onceOver: OnceOverState }>("/api/billing/once-over", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function refreshAuthSession() {
  return apiFetch<AuthSession>(
    "/api/auth/refresh",
    {
      method: "POST",
    },
    false,
  );
}

export async function login(email: string, name?: string) {
  return apiFetch<{ status: string; message: string }>("/api/auth/magic-link", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ email, name }),
  });
}

export async function startAnonymousSession() {
  return apiFetch<SessionResponse>("/api/sessions/anonymous", {
    method: "POST",
  }).then((response) => response.session);
}

export async function restoreWorkspace(key: string) {
  return apiFetch<SessionResponse>("/api/sessions/restore", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ key }),
  }).then((response) => response.session);
}

export async function createRecoveryKey() {
  return apiFetch<{ recoveryUrl: string }>("/api/sessions/recovery-key", {
    method: "POST",
  });
}

export async function revokeRecoveryKey() {
  return apiFetch<{ status: string }>("/api/sessions/recovery-key", {
    method: "DELETE",
  });
}

export async function claimWorkspace(email: string, name?: string) {
  return apiFetch<{ session: BuilderSession; status: string }>(
    "/api/sessions/claim",
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ email, name }),
    },
  );
}

export async function logout() {
  return apiFetch<{ status: string }>("/api/auth/logout", {
    method: "POST",
  });
}

export async function listSites() {
  return apiFetch<{ sites: SiteSummary[] }>("/api/sites");
}

export async function getSiteDraft(siteId: string) {
  return apiFetch<SiteDraftResponse>(`/api/sites/${siteId}`);
}

export async function createPreviewToken(siteId: string) {
  return apiFetch<PreviewTokenResponse>(`/api/sites/${siteId}/preview-token`, {
    method: "POST",
  });
}

export async function revokePreviewToken(siteId: string) {
  return apiFetch<void>(`/api/sites/${siteId}/preview-token`, {
    method: "DELETE",
  });
}

export async function getPreviewDraft(token: string) {
  return publicAPIRequest<PreviewDraftResponse>(`/api/public/preview/${token}`);
}

export async function createSite(input: {
  name: string;
  prompt?: string;
  slug?: string;
}) {
  return apiFetch<{ draft: SiteDraft }>("/api/sites", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function generateSite(input: {
  name?: string;
  prompt: string;
  slug?: string;
  preferredLanguage?: string;
  optionalHints?: Record<string, string>;
  brand?: BrandConfig;
}) {
  return apiFetch<SiteRepromptResponse>("/api/sites/generate", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function streamGenerateSite(
  input: {
    name?: string;
    prompt: string;
    slug?: string;
    preferredLanguage?: string;
    optionalHints?: Record<string, string>;
    brand?: BrandConfig;
  },
  handlers?: {
    onJobCreated?: (jobId: string) => void;
    onProgress?: (step: GenerationProgressStep) => void;
  },
) {
  return streamAPIRequest(
    "/api/sites/generate",
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
    handlers,
  );
}

export async function repromptSite(siteId: string, input: { prompt: string }) {
  return apiFetch<SiteRepromptResponse>(`/api/sites/${siteId}/reprompt`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function streamRepromptSite(
  siteId: string,
  input: { prompt: string },
  handlers?: {
    onJobCreated?: (jobId: string) => void;
    onProgress?: (step: GenerationProgressStep) => void;
  },
) {
  return streamAPIRequest(
    `/api/sites/${siteId}/reprompt`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
    handlers,
  );
}

export async function repromptPage(
  siteId: string,
  pageId: string,
  input: { prompt: string },
) {
  return apiFetch<SiteRepromptResponse>(
    `/api/sites/${siteId}/pages/${pageId}/reprompt`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
  );
}

export async function streamRepromptPage(
  siteId: string,
  pageId: string,
  input: { prompt: string },
  handlers?: {
    onJobCreated?: (jobId: string) => void;
    onProgress?: (step: GenerationProgressStep) => void;
  },
) {
  return streamAPIRequest(
    `/api/sites/${siteId}/pages/${pageId}/reprompt`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
    handlers,
  );
}

export async function getGenerationJob(jobId: string) {
  return apiFetch<{ job: GenerationJob }>(`/api/generation/jobs/${jobId}`);
}

export async function undoSiteReprompt(siteId: string) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/undo`, {
    method: "POST",
  });
}

export async function listRepromptHistory(siteId: string) {
  return apiFetch<{ reprompts: RepromptHistoryRecord[] }>(
    `/api/sites/${siteId}/reprompts`,
  );
}

export async function getDraftRevision(siteId: string, revisionId: string) {
  return apiFetch<{ revision: DraftRevisionRecord }>(
    `/api/sites/${siteId}/revisions/${revisionId}`,
  );
}

export async function revertReprompt(siteId: string, repromptId: string) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/reprompts/${repromptId}/revert`,
    {
      method: "POST",
    },
  );
}

export async function updateSite(
  siteId: string,
  input: {
    name?: string;
    slug?: string;
    brand?: BrandConfig;
  },
) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function getSiteTheme(siteId: string) {
  return apiFetch<ThemeState>(`/api/sites/${siteId}/theme`);
}

export async function updateSiteTheme(
  siteId: string,
  input: Partial<ThemeSelection>,
) {
  return apiFetch<ThemeState>(`/api/sites/${siteId}/theme`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function regenerateSiteTheme(siteId: string) {
  return apiFetch<ThemeState>(`/api/sites/${siteId}/theme/regenerate`, {
    method: "POST",
  });
}

export async function streamRegenerateSiteTheme(
  siteId: string,
  handlers?: {
    onJobCreated?: (jobId: string) => void;
    onProgress?: (step: GenerationProgressStep) => void;
  },
) {
  await streamAPIRequest(
    `/api/sites/${siteId}/theme/regenerate`,
    {
      method: "POST",
    },
    handlers,
  );
  return getSiteTheme(siteId);
}

export async function createAssetUploadURL(input: {
  siteId: string;
  fileName: string;
  contentType: string;
  sizeBytes: number;
  kind?: string;
  altText?: string;
}) {
  return apiFetch<AssetUploadTicket>("/api/assets/upload-url", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function completeAssetUpload(
  assetId: string,
  input: {
    altText?: string;
    width?: number;
    height?: number;
  } = {},
) {
  return apiFetch<{ asset: AssetRecord }>("/api/assets/complete", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ assetId, ...input }),
  });
}

export async function listSiteAssets(siteId: string) {
  return apiFetch<{ assets: AssetRecord[] }>(`/api/sites/${siteId}/assets`);
}

export async function listSiteFormSubmissions(siteId: string) {
  return apiFetch<{ submissions: FormSubmissionRecord[] }>(
    `/api/sites/${siteId}/form-submissions`,
  );
}

export async function updateFormSubmission(
  submissionId: string,
  input: {
    status: FormSubmissionStatus;
  },
) {
  return apiFetch<{ submission: FormSubmissionRecord }>(
    `/api/form-submissions/${submissionId}`,
    {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
  );
}

export async function updateAsset(
  assetId: string,
  input: {
    altText?: string;
  },
) {
  return apiFetch<{ asset: AssetRecord }>(`/api/assets/${assetId}`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function deleteAsset(assetId: string) {
  return apiFetch<void>(`/api/assets/${assetId}`, {
    method: "DELETE",
  });
}

export async function createPage(
  siteId: string,
  input: {
    title: string;
    slug?: string;
    status?: "draft" | "published";
    type?: PageType;
    collectionId?: string;
    includeInNavigation?: boolean;
  },
) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/pages`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function updatePage(
  siteId: string,
  pageId: string,
  input: {
    title?: string;
    slug?: string;
    status?: "draft" | "published";
    type?: PageType;
    collectionId?: string;
    seo?: {
      title?: string;
      description?: string;
    };
    includeInNavigation?: boolean;
  },
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}`,
    {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
  );
}

export async function deletePage(siteId: string, pageId: string) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}`,
    {
      method: "DELETE",
    },
  );
}

export async function reorderPages(siteId: string, pageIds: string[]) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/pages/reorder`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ pageIds }),
  });
}

export async function reorderSiteNavigation(siteId: string, pageIds: string[]) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/navigation/reorder`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ pageIds }),
    },
  );
}

export type NavigationItemInput = {
  label: string;
  pageId?: string;
  href?: string;
};

export async function updateSiteNavigation(
  siteId: string,
  navigation: {
    primary: NavigationItemInput[];
    footer: NavigationItemInput[];
  },
) {
  return apiFetch<{ draft: SiteDraft }>(`/api/sites/${siteId}/navigation`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(navigation),
  });
}

export async function createBlock(
  siteId: string,
  pageId: string,
  input: {
    type: string;
    version?: string;
  },
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
  );
}

export async function deleteSite(siteId: string) {
  return apiFetch<unknown>(`/api/sites/${siteId}`, {
    method: "DELETE",
  });
}

export async function updateBlock(
  siteId: string,
  pageId: string,
  blockId: string,
  input: {
    props?: Record<string, unknown>;
    hidden?: boolean;
  },
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/${blockId}`,
    {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(input),
    },
  );
}

export async function deleteBlock(
  siteId: string,
  pageId: string,
  blockId: string,
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/${blockId}`,
    {
      method: "DELETE",
    },
  );
}

export async function duplicateBlock(
  siteId: string,
  pageId: string,
  blockId: string,
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/${blockId}/duplicate`,
    {
      method: "POST",
    },
  );
}

export async function reorderBlocks(
  siteId: string,
  pageId: string,
  blockIds: string[],
) {
  return apiFetch<{ draft: SiteDraft }>(
    `/api/sites/${siteId}/pages/${pageId}/blocks/reorder`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ blockIds }),
    },
  );
}

export async function publishSite(
  siteId: string,
  input: {
    publishNote?: string;
  } = {},
) {
  return apiFetch<PublishSiteResponse>(`/api/sites/${siteId}/publish`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
}

export async function listSiteVersions(siteId: string) {
  return apiFetch<SiteVersionsResponse>(`/api/sites/${siteId}/versions`);
}

export type AnalyticsWindow = "7d" | "30d" | "all";

export type SiteAnalyticsPage = {
  pageId: string;
  title?: string;
  slug?: string;
  viewCount: number;
};

export type SiteAnalyticsDaily = {
  date: string;
  viewCount: number;
};

export type SiteAnalytics = {
  siteId: string;
  window: AnalyticsWindow;
  rangeStart?: string;
  rangeEnd?: string;
  totalViews: number;
  pages: SiteAnalyticsPage[];
  dailyViews: SiteAnalyticsDaily[];
};

export async function getSiteAnalytics(
  siteId: string,
  window: AnalyticsWindow = "7d",
) {
  const search = new URLSearchParams({ window });
  return apiFetch<{ analytics: SiteAnalytics }>(
    `/api/sites/${siteId}/analytics?${search.toString()}`,
  );
}

export async function getSiteDomains(siteId: string) {
  return apiFetch<SiteDomainsResponse>(`/api/sites/${siteId}/domains`);
}

export async function createSiteDomain(
  siteId: string,
  input: { hostname: string },
) {
  return apiFetch<SiteDomainsResponse>(`/api/sites/${siteId}/domains`, {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export async function verifySiteDomain(siteId: string, domainId: string) {
  return apiFetch<SiteDomainsResponse>(
    `/api/sites/${siteId}/domains/${domainId}`,
    {
      method: 'PATCH',
      body: JSON.stringify({ action: 'verify' }),
    },
  );
}

export async function deleteSiteDomain(siteId: string, domainId: string) {
  return apiFetch<SiteDomainsResponse>(
    `/api/sites/${siteId}/domains/${domainId}`,
    {
      method: 'DELETE',
    },
  );
}

export async function rollbackSiteVersion(siteId: string, versionId: string) {
  return apiFetch<RollbackSiteResponse>(
    `/api/sites/${siteId}/rollback/${versionId}`,
    {
      method: "POST",
    },
  );
}

export async function getPublishedSite(siteSlug: string, pagePath = "/") {
  const search = new URLSearchParams();
  if (pagePath && pagePath !== "/") {
    search.set("path", pagePath);
  }

  const suffix = search.size > 0 ? `?${search.toString()}` : "";
  return publicAPIRequest<PublishedSiteResponse>(
    `/api/public/sites/${siteSlug}${suffix}`,
  );
}

export async function getPublishedSiteByHostname(
  hostname: string,
  pagePath = "/",
) {
  const search = new URLSearchParams({ hostname });
  if (pagePath && pagePath !== "/") {
    search.set("path", pagePath);
  }

  return publicAPIRequest<PublishedSiteResponse>(
    `/api/public/render?${search.toString()}`,
  );
}

export async function getPublishedArtifact(input: {
  siteSlug?: string;
  hostname?: string;
  path: string;
}) {
  const search = new URLSearchParams({ path: input.path });
  if (input.hostname) {
    search.set("hostname", input.hostname);
  }
  if (input.siteSlug) {
    search.set("siteSlug", input.siteSlug);
  }

  const response = await fetch(
    new URL(`/api/public/artifact?${search.toString()}`, getAPIBaseURL()),
    {
      credentials: "include",
      headers: {
        Accept: "text/plain, application/xml, text/css, application/json",
      },
    },
  );

  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new APIError(response.status, payload);
  }

  return response.text();
}

export async function submitPublicForm(
  siteId: string,
  blockId: string,
  payload: Record<string, unknown>,
) {
  return publicAPIRequest<{ status: string; message: string }>(
    `/api/public/forms/${siteId}/${blockId}/submit`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ payload }),
    },
  );
}

// ---------------------------------------------------------------------------
// Collections API
// ---------------------------------------------------------------------------

export async function listCollections(siteId: string) {
  return apiFetch<{ collections: Collection[] }>(
    `/api/sites/${siteId}/collections`,
    { method: "GET" },
  );
}

export async function getCollection(siteId: string, collectionId: string) {
  return apiFetch<{ collection: Collection }>(
    `/api/sites/${siteId}/collections/${collectionId}`,
    { method: "GET" },
  );
}

export async function createCollection(
  siteId: string,
  input: {
    slug?: string;
    singularLabel: string;
    pluralLabel: string;
    schema?: FieldDefinition[];
    settings?: CollectionSettings;
  },
) {
  return apiFetch<{ collection: Collection }>(
    `/api/sites/${siteId}/collections`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
}

export async function updateCollection(
  siteId: string,
  collectionId: string,
  input: {
    slug?: string;
    singularLabel?: string;
    pluralLabel?: string;
    schema?: FieldDefinition[];
    settings?: CollectionSettings;
  },
) {
  return apiFetch<{ collection: Collection }>(
    `/api/sites/${siteId}/collections/${collectionId}`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
}

export async function deleteCollection(siteId: string, collectionId: string) {
  return apiFetch<null>(`/api/sites/${siteId}/collections/${collectionId}`, {
    method: "DELETE",
  });
}

export async function listCollectionEntries(
  siteId: string,
  collectionId: string,
) {
  return apiFetch<{ entries: CollectionEntry[] }>(
    `/api/sites/${siteId}/collections/${collectionId}/entries`,
    { method: "GET" },
  );
}

export async function getCollectionEntry(
  siteId: string,
  collectionId: string,
  entryId: string,
) {
  return apiFetch<{ entry: CollectionEntry }>(
    `/api/sites/${siteId}/collections/${collectionId}/entries/${entryId}`,
    { method: "GET" },
  );
}

export async function createCollectionEntry(
  siteId: string,
  collectionId: string,
  input: {
    slug?: string;
    fields?: Record<string, unknown>;
    seo?: { title?: string; description?: string };
    status?: "draft" | "published";
  },
) {
  return apiFetch<{ entry: CollectionEntry }>(
    `/api/sites/${siteId}/collections/${collectionId}/entries`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
}

export async function updateCollectionEntry(
  siteId: string,
  collectionId: string,
  entryId: string,
  input: {
    slug?: string;
    fields?: Record<string, unknown>;
    seo?: { title?: string; description?: string };
    status?: "draft" | "published";
  },
) {
  return apiFetch<{ entry: CollectionEntry }>(
    `/api/sites/${siteId}/collections/${collectionId}/entries/${entryId}`,
    {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
}

export async function deleteCollectionEntry(
  siteId: string,
  collectionId: string,
  entryId: string,
) {
  return apiFetch<null>(
    `/api/sites/${siteId}/collections/${collectionId}/entries/${entryId}`,
    { method: "DELETE" },
  );
}

export async function reorderCollectionEntries(
  siteId: string,
  collectionId: string,
  entryIds: string[],
) {
  return apiFetch<{ entries: CollectionEntry[] }>(
    `/api/sites/${siteId}/collections/${collectionId}/entries/reorder`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ entryIds }),
    },
  );
}
