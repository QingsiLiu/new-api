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

import type { Model } from '../types'
import { parseModelTags as parseTagsFromUtils } from './model-utils'

// ============================================================================
// Model Form Schema
// ============================================================================

/**
 * Model form validation schema
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
  enable_groups: z.array(z.string()),
  quota_types: z.array(z.number()),
})

export type ModelFormValues = z.infer<typeof modelFormSchema>

// ============================================================================
// Vendor Form Schema
// ============================================================================

/**
 * Vendor form validation schema
 */
export const vendorFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().min(1, 'Vendor name is required'),
  description: z.string().default(''),
  icon: z.string().default(''),
  status: z.number().default(1),
})

export type VendorFormValues = z.infer<typeof vendorFormSchema>

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform model to form default values
 */
export function transformModelToFormDefaults(model: Model): ModelFormValues {
  return {
    id: model.id,
    model_name: model.model_name,
    alias: model.alias || '',
    description: model.description || '',
    icon: model.icon || '',
    tags: parseTagsFromUtils(model.tags),
    vendor_id: model.vendor_id,
    endpoints: model.endpoints || '',
    name_rule: model.name_rule || 0,
    status: model.status === 1,
    sync_official: model.sync_official === 1,
    modal: model.modal || 'text',
    text_category: model.text_category || 'unclassified',
    official_price_key: model.official_price_key || '',
    pricing_mode: model.pricing_mode || 'inherit',
    pricing_config: model.pricing_config || '',
    pricing_updated_time: model.pricing_updated_time || 0,
    enable_groups: model.enable_groups || [],
    quota_types: model.quota_types || [],
  }
}

/**
 * Transform form data to model create/update payload
 */
export function transformFormDataToModelPayload(
  formData: ModelFormValues
): Partial<Model> {
  return {
    id: formData.id,
    model_name: formData.model_name,
    alias: formData.alias || '',
    description: formData.description || '',
    icon: formData.icon || '',
    tags: formatTagsArray(formData.tags),
    vendor_id: formData.vendor_id,
    endpoints: formData.endpoints || '',
    name_rule: formData.name_rule,
    status: formData.status ? 1 : 0,
    sync_official: formData.sync_official ? 1 : 0,
    modal: formData.modal,
    text_category: formData.text_category,
    official_price_key: formData.official_price_key,
    pricing_mode: formData.pricing_mode,
    pricing_config: formData.pricing_config,
    pricing_updated_time: formData.pricing_updated_time,
    enable_groups: formData.enable_groups,
    quota_types: formData.quota_types,
  }
}

// ============================================================================
// Parsing and Formatting Helpers
// ============================================================================

/**
 * Format tags array to string
 */
export function formatTagsArray(tags: string[]): string {
  return tags.filter(Boolean).join(',')
}

/**
 * Validate JSON string
 */
export function validateJSON(value: string): boolean {
  if (!value || value.trim() === '') return true

  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

/**
 * Validate endpoints JSON
 */
export function validateEndpoints(endpoints: string): boolean {
  return validateJSON(endpoints)
}
