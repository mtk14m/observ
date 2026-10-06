import { afterEach } from 'vitest'
import { enableAutoUnmount } from '@vue/test-utils'

// Every mounted component is unmounted after its test, so tests stay isolated.
enableAutoUnmount(afterEach)
