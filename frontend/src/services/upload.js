import { API_BASE_URL } from './api'

/**
 * Upload service for handling file uploads to R2 via presigned URLs
 */

export class UploadService {
  /**
   * Initialize upload and get presigned URL
   */
  static async initUpload(conversationId, file) {
    const token = localStorage.getItem('token')
    if (!token) throw new Error('Not authenticated')

    const response = await fetch(`${API_BASE_URL}/uploads/init`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        conversation_id: conversationId,
        filename: file.name,
        mime_type: file.type,
        size_bytes: file.size,
      }),
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || 'Failed to initialize upload')
    }

    const initData = await response.json()
    console.log('Init upload response:', initData)
    return initData
  }

  /**
   * Upload file to R2 using presigned URL
   */
  static async uploadToR2(presignedUrl, file, onProgress) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest()

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable && onProgress) {
          const percentComplete = (e.loaded / e.total) * 100
          onProgress(percentComplete)
        }
      })

      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve()
        } else {
          reject(new Error(`Upload failed with status ${xhr.status}`))
        }
      })

      xhr.addEventListener('error', () => {
        reject(new Error('Network error during upload'))
      })

      xhr.open('PUT', presignedUrl)
      xhr.setRequestHeader('Content-Type', file.type)
      xhr.send(file)
    })
  }

  /**
   * Mark upload as complete
   */
  static async completeUpload(attachmentId) {
    const token = localStorage.getItem('token')
    if (!token) throw new Error('Not authenticated')

    const response = await fetch(`${API_BASE_URL}/uploads/complete`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        attachment_id: attachmentId,
      }),
    })

    if (!response.ok) {
      const text = await response.text()
      console.error('Complete upload failed:', text)
      let errorMessage = 'Failed to complete upload'
      try {
        const error = JSON.parse(text)
        errorMessage = error.error || error.message || errorMessage
        if (error.details) {
          console.error('Error details:', error.details)
        }
      } catch (_e) {
        errorMessage = text || `Upload failed with status ${response.status}`
      }
      throw new Error(errorMessage)
    }

    return response.json()
  }

  /**
   * Get download URL for attachment
   */
  static async getAttachmentUrl(attachmentId) {
    const token = localStorage.getItem('token')
    if (!token) throw new Error('Not authenticated')

    const response = await fetch(`${API_BASE_URL}/attachments/${attachmentId}/url`, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    })

    if (!response.ok) {
      const text = await response.text()
      let errorMessage = 'Failed to get attachment URL'
      try {
        const error = JSON.parse(text)
        errorMessage = error.error || error.message || errorMessage
      } catch (_e) {
        errorMessage = text || `Request failed with status ${response.status}`
      }
      throw new Error(errorMessage)
    }

    return response.json()
  }

  /**
   * Full upload flow: init -> upload -> complete
   */
  static async uploadFile(conversationId, file, onProgress) {
    // Step 1: Initialize upload
    const initData = await this.initUpload(conversationId, file)

    // Step 2: Upload to R2
    await this.uploadToR2(initData.presigned_url, file, onProgress)

    // Step 3: Mark complete
    console.log('Completing upload with attachment_id:', initData.attachment_id)
    await this.completeUpload(initData.attachment_id)

    return {
      attachmentId: initData.attachment_id,
      objectKey: initData.object_key,
    }
  }

  /**
   * Validate file before upload
   */
  static validateFile(file, maxSizeBytes = 50 * 1024 * 1024) {
    // 50MB default
    const allowedTypes = [
      'image/',
      'video/',
      'audio/',
      'application/pdf',
      'application/msword',
      'application/vnd.openxmlformats-officedocument',
      'text/',
    ]

    if (file.size > maxSizeBytes) {
      return {
        valid: false,
        error: `File size exceeds ${Math.round(maxSizeBytes / 1024 / 1024)}MB limit`,
      }
    }

    const isAllowed = allowedTypes.some((type) => file.type.startsWith(type))
    if (!isAllowed) {
      return {
        valid: false,
        error: 'File type not allowed',
      }
    }

    return { valid: true }
  }
}
