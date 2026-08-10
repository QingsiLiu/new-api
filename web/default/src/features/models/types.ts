/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { z } from 'zod'

// ============================================================================
// Model Types
// ============================================================================

/**
 * Bound channel information
 */
export interface BoundChannel {
  id: number
  name: string
  type: number
}

/**
 * Model entity from API
 */
export interface Model {
  id: number
  model_name: string
  alias?: string
  description?: string
  icon?: string
  tags?: string
  vendor_id?: number
  endpoints?: string
  status: number
  sync_official: number
  modal?: string
  text_category?: TextModelCategory | string
  official_price_key?: string
  text_multiplier_override?: number
  pricing_mode?: string
  pricing_config?: string
  pricing_updated_time?: number
  pricing_ready?: boolean
  pricing_error?: string
  official_price_profile?: OfficialPriceProfile | null
  effective_text_pricing?: EffectiveTextPricing | null
  created_time: number
  updated_time: number
  name_rule: number
  // Runtime fields
  bound_channels?: BoundChannel[]
  enable_groups?: string[]
  quota_types?: number[]
  matched_models?: string[]
  matched_count?: number
}

/**
 * Vendor entity from API
 */
export interface Vendor {
  id: number
  name: string
  description?: string
  icon?: string
  status: number
  created_time: number
  updated_time: number
}

/**
 * Prefill group entity
 */
export interface PrefillGroup {
  id: number
  name: string
  type: 'model' | 'tag' | 'endpoint'
  items: string | string[]
  description?: string
}

// ============================================================================
// API Request/Response Types
// ============================================================================

/**
 * Get models list parameters
 */
export interface GetModelsParams {
  p?: number
  page_size?: number
  vendor?: string // vendor ID to filter by
  status?: string // filter by status
  sync_official?: string // filter by sync_official status
  modal?: string // filter by model modality
  text_category?: string // filter text models by pricing category
  pricing_mode?: string // filter by pricing mode
  text_pricing_status?: 'pending' // filter unresolved text pricing metadata
}

/**
 * Search models parameters
 */
export interface SearchModelsParams {
  keyword?: string
  vendor?: string // vendor ID to filter by
  status?: string // filter by status
  sync_official?: string // filter by sync_official status
  modal?: string // filter by model modality
  text_category?: string // filter text models by pricing category
  pricing_mode?: string // filter by pricing mode
  text_pricing_status?: 'pending' // filter unresolved text pricing metadata
  p?: number
  page_size?: number
}

/**
 * Get models response
 */
export interface GetModelsResponse {
  success: boolean
  message?: string
  data?: {
    items: Model[]
    total: number
    page: number
    page_size: number
    vendor_counts?: Record<string, number>
  }
}

/**
 * Get model detail response
 */
export interface GetModelResponse {
  success: boolean
  message?: string
  data?: Model
}

/**
 * Get vendors response
 */
export interface GetVendorsResponse {
  success: boolean
  message?: string
  data?: {
    items: Vendor[]
    total: number
    page: number
    page_size: number
  }
}

/**
 * Get vendor response
 */
export interface GetVendorResponse {
  success: boolean
  message?: string
  data?: Vendor
}

/**
 * Sync diff data
 */
export interface SyncDiffData {
  missing?: Array<{
    model_name: string
    vendor?: string
    [key: string]: unknown
  }>
  conflicts?: Array<{
    model_name: string
    local?: Partial<Model>
    upstream?: Partial<Model>
    fields?: Array<{
      field: string
      local?: unknown
      upstream?: unknown
    }>
    [key: string]: unknown
  }>
}

export interface SyncOverwritePayload {
  model_name: string
  fields: string[]
}

/**
 * Sync upstream response
 */
export interface SyncUpstreamResponse {
  success: boolean
  message?: string
  data?: {
    created_models?: number
    updated_models?: number
    created_vendors?: number
    skipped_models?: string[]
  }
}

/**
 * Preview upstream diff response
 */
export interface PreviewUpstreamDiffResponse {
  success: boolean
  message?: string
  data?: SyncDiffData
}

/**
 * Missing models response
 */
export interface MissingModelsResponse {
  success: boolean
  message?: string
  data?: string[]
}

/**
 * Prefill groups response
 */
export interface PrefillGroupsResponse {
  success: boolean
  message?: string
  data?: PrefillGroup[]
}

// ============================================================================
// Text Pricing Types
// ============================================================================

export const TEXT_MODEL_CATEGORIES = [
  'gpt',
  'claude',
  'gemini',
  'grok',
  'unclassified',
] as const

export type TextModelCategory = (typeof TEXT_MODEL_CATEGORIES)[number]

export const TEXT_PRICING_GROUPS = ['gpt', 'claude', 'gemini', 'grok'] as const

export type TextPricingGroup = (typeof TEXT_PRICING_GROUPS)[number]

export interface OfficialPriceDimensions {
  input?: number
  output?: number
  cached_input?: number
  cache_write?: number
  cache_write_5m?: number
  cache_write_1h?: number
}

export interface OfficialPriceTier {
  label: string
  min_prompt_tokens?: number
  max_prompt_tokens?: number
  dimensions: OfficialPriceDimensions
}

export interface OfficialPriceProfile {
  key: string
  version?: string
  category: TextModelCategory | string
  display_name: string
  currency?: string
  unit?: string
  source_url?: string
  dimensions?: OfficialPriceDimensions
  tiers?: OfficialPriceTier[]
}

export interface EffectiveTextPricing {
  category?: TextModelCategory | string
  category_multiplier?: number
  model_multiplier_override?: number
  effective_multiplier?: number
  multiplier_source?: 'category' | 'model_override' | string
  catalog_version?: string
  official_price_key?: string
  pricing_source?: string
  input_quota_per_million?: number
  output_quota_per_million?: number
  cached_input_quota_per_million?: number
  cache_write_quota_per_million?: number
  cache_write_5m_quota_per_million?: number
  cache_write_1h_quota_per_million?: number
  dimensions?: Record<string, number>
  tiers?: EffectiveTextPricingTier[]
}

export interface EffectiveTextPricingTier {
  label: string
  min_prompt_tokens?: number
  max_prompt_tokens?: number
  input_quota_per_million: number
  output_quota_per_million: number
  cached_input_quota_per_million?: number
  cache_write_quota_per_million?: number
  cache_write_5m_quota_per_million?: number
  cache_write_1h_quota_per_million?: number
}

export interface TextPricingCategoryConfig {
  category: TextModelCategory | string
  multiplier?: number
  model_count?: number
  pricing_ready_count?: number
  pricing_blocked_count?: number
  override_count?: number
  inherited_count?: number
  catalog_profile_count?: number
  activation_ready?: boolean
  activation_error?: string
  updated_time?: number
}

export type TextPricingCategoriesPayload =
  | TextPricingCategoryConfig[]
  | Record<string, number | TextPricingCategoryConfig>

export type TextPricingProfilesPayload =
  | OfficialPriceProfile[]
  | Record<string, OfficialPriceProfile>

export type TextPricingMode = 'legacy' | 'shadow' | 'active'

export interface TextPricingConfig {
  mode: TextPricingMode | string
  catalog_version: string
  categories: TextPricingCategoriesPayload
  profiles: TextPricingProfilesPayload
  pending_count?: number
  unclassified_count?: number
  missing_official_profile_count?: number
  activation_ready?: boolean
  activation_blockers?: string[]
}

export interface TextPricingConfigResponse {
  success: boolean
  message?: string
  data?: TextPricingConfig
}

export interface TextPricingImpact {
  id: number
  model_name: string
  official_price_key: string
  category_multiplier?: number
  model_multiplier_override?: number
  effective_multiplier?: number
  multiplier_source?: 'category' | 'model_override' | string
  input_quota_per_million: number
  output_quota_per_million: number
  pricing_ready: boolean
  affected?: boolean
  pricing_error?: string
}

export interface TextPricingPreviewSummary {
  category: TextModelCategory | string
  multiplier?: number
  models: TextPricingImpact[]
}

export interface TextPricingPreview {
  affected_count: number
  override_count?: number
  before: TextPricingPreviewSummary
  after: TextPricingPreviewSummary
}

export interface TextPricingPreviewResponse {
  success: boolean
  message?: string
  data?: TextPricingPreview
}

export interface UpdateTextPricingCategoryResponse {
  success: boolean
  message?: string
  data?: TextPricingCategoryConfig
}

export interface UpdateTextPricingModeResponse {
  success: boolean
  message?: string
  data?: { mode: TextPricingMode | string }
}

export interface TextPricingModelPreview {
  model_id: number
  model_name: string
  before: TextPricingImpact
  after: TextPricingImpact
}

export interface TextPricingModelPreviewResponse {
  success: boolean
  message?: string
  data?: TextPricingModelPreview
}

export type UpdateTextPricingModelResponse = TextPricingModelPreviewResponse

// ============================================================================
// Form Data Types
// ============================================================================

/**
 * Model form schema
 */
export const modelFormSchema = z.object({
  id: z.number().optional(),
  model_name: z.string().min(1, 'Model name is required'),
  alias: z.string(),
  description: z.string(),
  icon: z.string(),
  tags: z.array(z.string()),
  vendor_id: z.number().optional(),
  endpoints: z.string(),
  name_rule: z.number().min(0).max(3),
  status: z.boolean(),
  sync_official: z.boolean(),
  modal: z.string(),
  text_category: z.string(),
  official_price_key: z.string(),
  pricing_mode: z.string(),
  pricing_config: z.string(),
  pricing_updated_time: z.number(),
})

export type ModelFormValues = z.infer<typeof modelFormSchema>

/**
 * Vendor form schema
 */
export const vendorFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().min(1, 'Vendor name is required'),
  description: z.string().default(''),
  icon: z.string().default(''),
  status: z.number().default(1),
})

export type VendorFormValues = z.infer<typeof vendorFormSchema>

/**
 * Prefill group form schema
 */
export const prefillGroupFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().min(1, 'Group name is required'),
  description: z.string().optional(),
  type: z.enum(['model', 'tag', 'endpoint']),
  items: z.union([z.string(), z.array(z.string())]),
})

export type PrefillGroupFormValues = z.infer<typeof prefillGroupFormSchema>

// ============================================================================
// Utility Types
// ============================================================================

/**
 * Name rule type
 */
export type NameRule = 0 | 1 | 2 | 3 // exact, prefix, contains, suffix

/**
 * Model status type
 */
export type ModelStatus = 0 | 1 // disabled, enabled

/**
 * Quota type
 */
export type QuotaType = 0 | 1 // usage-based, per-call

/**
 * Sync locale
 */
export type SyncLocale = 'zh' | 'en' | 'ja'

/**
 * Sync upstream source
 */
export type SyncSource = 'official' | 'config'

// ============================================================================
// Model Deployments Types
// ============================================================================

/**
 * Model tab type
 */
export type ModelTabCategory = 'metadata' | 'deployments'

/**
 * Deployment entity from API
 */
export interface Deployment {
  id: string | number
  container_name?: string
  deployment_name?: string
  name?: string
  status?: string
  provider?: string
  /**
   * Human readable string returned by backend, e.g. "2 hour 15 minutes"
   * or "completed".
   */
  time_remaining?: string
  /**
   * Remaining minutes (numeric) returned by backend.
   */
  compute_minutes_remaining?: number
  /**
   * Served minutes (numeric) returned by backend.
   */
  compute_minutes_served?: number
  /**
   * Completed percent (0-100) returned by backend.
   */
  completed_percent?: number
  hardware_info?: string | Record<string, unknown>
  hardware_name?: string
  brand_name?: string
  hardware_quantity?: number
  created_at?: string | number
  updated_at?: string | number
  [key: string]: unknown
}

/**
 * Deployment settings response
 */
export interface DeploymentSettingsResponse {
  success: boolean
  message?: string
  data?: {
    enabled?: boolean
    [key: string]: unknown
  }
}

/**
 * List deployments response
 */
export interface ListDeploymentsResponse {
  success: boolean
  message?: string
  data?: {
    items?: Deployment[]
    total?: number
    page?: number
    page_size?: number
    status_counts?: Record<string, number>
  }
}

/**
 * Deployment logs response
 */
export interface DeploymentLogsResponse {
  success: boolean
  message?: string
  data?: {
    logs?: Array<{
      timestamp?: string
      level?: string
      message?: string
      source?: string
    }>
    cursor?: string
  }
}
