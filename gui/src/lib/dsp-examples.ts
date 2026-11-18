// Canonical JSON-LD message examples from the Eclipse Dataspace Protocol 2025-1-err2 specification.
// Used by the client Explain console to populate protocol-compliant paths and payloads per rule.
import type { Rule } from '@/lib/client'

// DSP_CONTEXT is the normative JSON-LD context URL of Dataspace Protocol 2025-1.
export const DSP_CONTEXT = 'https://w3id.org/dspace/2025/1/context.jsonld'

// PROVIDER_PID is the provider process identifier from the DSP 2025-1-err2 examples.
export const PROVIDER_PID = 'urn:uuid:a343fcbf-99fc-4ce8-8e9b-148c97605aab'

// CONSUMER_PID is the consumer process identifier from the DSP 2025-1-err2 examples.
export const CONSUMER_PID = 'urn:uuid:32541fe6-c580-409e-85a8-8a9a32fbe833'

// DATASET_ID is the dataset identifier from the DSP 2025-1-err2 catalog examples.
export const DATASET_ID = 'urn:uuid:3afeadd8-ed2d-569e-d634-8394a8836d57'

// DSP_DATETIME_EXAMPLE is the canonical ISO-8601 dateTime string used in DSP 2025-1-err2 examples.
const DSP_DATETIME_EXAMPLE = '2023-12-31T06:00Z'

// ruleExamples maps seed rule identifiers to their exact DSP 2025-1-err2 specification payloads.
const ruleExamples: Record<string, Record<string, unknown>> = {
  'catalog-request': {
    '@context': [DSP_CONTEXT],
    '@type': 'CatalogRequestMessage',
    filter: [],
  },
  'contract-request-initial': {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractRequestMessage',
    consumerPid: CONSUMER_PID,
    offer: {
      '@type': 'Offer',
      '@id': 'urn:uuid:2828282:3dd1add8-4d2d-569e-d634-8394a8836a89',
      target: 'urn:uuid:3dd1add8-4d2d-569e-d634-8394a8836a88',
      permission: [{ action: 'use' }],
    },
    callbackAddress: 'https://example.com/callback',
  },
  'contract-request': {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractRequestMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    offer: {
      '@type': 'Offer',
      '@id': 'urn:uuid:d526561f-528e-4d5a-ae12-9a9dd9b7a815',
      target: 'urn:uuid:3dd1add8-4d2d-569e-d634-8394a8836a88',
      permission: [
        {
          action: 'use',
          constraint: [
            {
              leftOperand: 'dateTime',
              operator: 'lteq',
              rightOperand: DSP_DATETIME_EXAMPLE,
            },
          ],
        },
      ],
    },
  },
  'contract-offer-initial': {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractOfferMessage',
    providerPid: PROVIDER_PID,
    offer: {
      '@type': 'Offer',
      '@id': 'urn:uuid:d526561f-528e-4d5a-ae12-9a9dd9b7a518',
      target: 'urn:uuid:3dd1add8-4d2d-569e-d634-8394a8836a88',
      permission: [{ action: 'use' }],
    },
    callbackAddress: 'https://example.com/callback',
  },
  'contract-offer': {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractOfferMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    offer: {
      '@type': 'Offer',
      '@id': 'urn:uuid:6bcea82e-c509-443d-ba8c-8eef25984c07',
      target: 'urn:uuid:3dd1add8-4d2d-569e-d634-8394a8836a88',
      permission: [
        {
          action: 'use',
          constraint: [
            {
              leftOperand: 'dateTime',
              operator: 'lteq',
              rightOperand: DSP_DATETIME_EXAMPLE,
            },
          ],
        },
      ],
    },
  },
}

// messageExamples maps DSP 2025-1-err2 message types to their specification payloads.
const messageExamples: Record<string, Record<string, unknown>> = {
  CatalogRequestMessage: ruleExamples['catalog-request'],
  DatasetRequestMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'DatasetRequestMessage',
    dataset: DATASET_ID,
  },
  ContractRequestMessage: ruleExamples['contract-request'],
  ContractOfferMessage: ruleExamples['contract-offer'],
  ContractAgreementMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractAgreementMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    agreement: {
      '@id': 'urn:uuid:e8dc8655-44c2-46ef-b701-4cffdc2faa44',
      '@type': 'Agreement',
      target: 'urn:uuid:3dd1add4-4d2d-569e-d634-8394a8836d23',
      timestamp: '2023-01-01T01:00:00Z',
      assigner: 'http://example.com/Provider',
      assignee: 'http://example.com/Consumer',
      permission: [
        {
          action: 'use',
          constraint: [
            {
              leftOperand: 'dateTime',
              operator: 'lteq',
              rightOperand: DSP_DATETIME_EXAMPLE,
            },
          ],
        },
      ],
    },
  },
  ContractAgreementVerificationMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractAgreementVerificationMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
  },
  ContractNegotiationEventMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractNegotiationEventMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    eventType: 'ACCEPTED',
  },
  ContractNegotiationTerminationMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'ContractNegotiationTerminationMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    code: '99',
    reason: ['License model does not fit.'],
  },
  TransferRequestMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'TransferRequestMessage',
    consumerPid: CONSUMER_PID,
    agreementId: 'urn:uuid:e8dc8655-44c2-46ef-b701-4cffdc2faa44',
    format: 'example:HTTP_PUSH',
    dataAddress: {
      '@type': 'DataAddress',
      endpointType: 'https://w3id.org/idsa/v4.1/HTTP',
      endpoint: 'http://example.com',
      endpointProperties: [
        { '@type': 'EndpointProperty', name: 'authorization', value: 'TOKEN-ABCDEFG' },
        { '@type': 'EndpointProperty', name: 'authType', value: 'bearer' },
      ],
    },
    callbackAddress: 'https://example.com/callback',
  },
  TransferStartMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'TransferStartMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    dataAddress: {
      '@type': 'DataAddress',
      endpointType: 'https://w3id.org/idsa/v4.1/HTTP',
      endpoint: 'http://example.com',
      endpointProperties: [
        { '@type': 'EndpointProperty', name: 'authorization', value: 'TOKEN-ABCDEFG' },
        { '@type': 'EndpointProperty', name: 'authType', value: 'bearer' },
      ],
    },
  },
  TransferSuspensionMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'TransferSuspensionMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    code: '99',
    reason: ['Policy violation'],
  },
  TransferCompletionMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'TransferCompletionMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
  },
  TransferTerminationMessage: {
    '@context': [DSP_CONTEXT],
    '@type': 'TransferTerminationMessage',
    providerPid: PROVIDER_PID,
    consumerPid: CONSUMER_PID,
    code: '99',
    reason: ['Policy violation'],
  },
}

// DEFAULT_EXPLAIN_MESSAGE is the initial ContractRequestMessage shown in the Explain console.
export const DEFAULT_EXPLAIN_MESSAGE = ruleExamples['contract-request']

// concretizeDspPath replaces URI template variables with DSP 2025-1-err2 example identifiers.
export function concretizeDspPath(path: string): string {
  return path.replace(/\{([^}]+)\}/g, (_, key: string) => {
    if (key === 'consumerPid') return CONSUMER_PID
    if (key === 'id') return DATASET_ID
    return PROVIDER_PID
  })
}

// dspExampleForRule returns the DSP 2025-1-err2 request payload for a rule, or null for bodyless GET requests.
export function dspExampleForRule(rule: Rule): Record<string, unknown> | null {
  if (rule.match.method === 'GET') return null
  if (ruleExamples[rule.id]) return ruleExamples[rule.id]
  if (messageExamples[rule.message]) return messageExamples[rule.message]
  return { '@context': [DSP_CONTEXT], '@type': rule.message }
}
