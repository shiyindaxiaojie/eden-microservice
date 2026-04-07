import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '',
  timeout: 10000,
})

// Add JWT token to every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
}, (error) => {
  return Promise.reject(error)
})

// Handle 401 Unauthorized
api.interceptors.response.use((response) => {
  return response
}, (error) => {
  if (error.response?.status === 401) {
    // Don't redirect if we're already trying to login or already on login page
    const isLoginEndpoint = error.config.url.endsWith('/v1/auth/login')
    const isLoginPage = window.location.pathname === '/login'
    if (!isLoginEndpoint && !isLoginPage) {
      localStorage.removeItem('token')
      localStorage.removeItem('user_role')
      localStorage.removeItem('username')
      localStorage.removeItem('nickname')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
  }
  return Promise.reject(error)
})


export default api
