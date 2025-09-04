import { useMemo } from "react"
import { Configuration, DefaultApi } from "../generated_api/src"

const BASE_PATH = '/api/v1'

/**
 * @deprecated Use grpc methods instead.
 */
export const useApiClient = () => {
  return useMemo(() => {
    const config = new Configuration({ basePath: BASE_PATH, credentials: 'include' })
    return new DefaultApi(config)
  }, [])
}
