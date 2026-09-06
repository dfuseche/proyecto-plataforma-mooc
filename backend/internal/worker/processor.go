package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"
	"github.com/mooc-platform/backend/internal/domain"
	"github.com/mooc-platform/backend/internal/media"
)

type Processor struct {
	storage    *media.StorageService
	courseRepo domain.CourseRepository
}

func NewProcessor(storage *media.StorageService, courseRepo domain.CourseRepository) *Processor {
	return &Processor{
		storage:    storage,
		courseRepo: courseRepo,
	}
}

func (p *Processor) HandleMediaTranscodeHLS(ctx context.Context, t *asynq.Task) error {
	var payload MediaTranscodeHLSPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	log.Printf("[WORKER] Iniciando transcodificación HLS para el recurso %s (Key: %s)", payload.ResourceID, payload.OriginalKey)

	// Verificación de idempotencia: si la playlist HLS ya existe en S3, reutilizar y marcar completado
	hlsKeyPrefix := fmt.Sprintf("hls/%s/", payload.ResourceID.String())
	playlistKey := hlsKeyPrefix + "master.m3u8"

	minioClient := p.storage.GetClient()
	mediaBucket := p.storage.GetMediaBucket()

	_, err := minioClient.StatObject(ctx, mediaBucket, playlistKey, minio.StatObjectOptions{})
	if err == nil {
		log.Printf("[WORKER-IDEMPOTENCY] La playlist HLS %s ya existe en S3. Omitiendo transcodificación.", playlistKey)
		return p.markResourceCompleted(ctx, payload.ResourceID, playlistKey)
	}

	// Crear directorio temporal de trabajo local
	tmpDir, err := os.MkdirTemp("", "hls-transcode-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Descargar archivo original desde S3 a local
	localOriginalPath := filepath.Join(tmpDir, "original")
	err = minioClient.FGetObject(ctx, mediaBucket, payload.OriginalKey, localOriginalPath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to download original file from S3: %w", err)
	}

	// Ejecutar FFmpeg para generar HLS adaptativo sin upscaling
	outputPlaylist := filepath.Join(tmpDir, "master.m3u8")
	segmentFilename := filepath.Join(tmpDir, "segment_%03d.ts")

	var cmd *exec.Cmd
	if strings.ToLower(payload.MediaType) == "audio" {
		cmd = exec.CommandContext(ctx, "ffmpeg",
			"-i", localOriginalPath,
			"-c:a", "aac",
			"-b:a", "128k",
			"-f", "hls",
			"-hls_time", "10",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", segmentFilename,
			outputPlaylist,
		)
	} else {
		// Video: HLS sin upscaling
		cmd = exec.CommandContext(ctx, "ffmpeg",
			"-i", localOriginalPath,
			"-c:v", "h264",
			"-crf", "22",
			"-preset", "fast",
			"-c:a", "aac",
			"-b:a", "128k",
			"-f", "hls",
			"-hls_time", "10",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", segmentFilename,
			outputPlaylist,
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[WORKER-FFMPEG-ERROR] FFmpeg falló: %s, output: %s", err, string(output))
		return fmt.Errorf("ffmpeg transcoding failed: %w", err)
	}

	// Subir segmentos .ts y playlist .m3u8 generados a MinIO/S3
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() || file.Name() == "original" {
			continue
		}
		filePath := filepath.Join(tmpDir, file.Name())
		targetS3Key := hlsKeyPrefix + file.Name()

		contentType := "video/MP2T"
		if strings.HasSuffix(file.Name(), ".m3u8") {
			contentType = "application/x-mpegURL"
		}

		_, err = minioClient.FPutObject(ctx, mediaBucket, targetS3Key, filePath, minio.PutObjectOptions{
			ContentType: contentType,
		})
		if err != nil {
			return fmt.Errorf("failed to upload HLS segment %s to S3: %w", file.Name(), err)
		}
	}

	log.Printf("[WORKER-SUCCESS] Transcodificación HLS completada para %s. Playlist: %s", payload.ResourceID, playlistKey)
	return p.markResourceCompleted(ctx, payload.ResourceID, playlistKey)
}

func (p *Processor) markResourceCompleted(ctx context.Context, resourceID uuid.UUID, mediaURL string) error {
	res := &domain.Resource{
		ID:               resourceID,
		MediaURL:         mediaURL,
		ProcessingStatus: domain.ProcessingCompleted,
	}
	return p.courseRepo.UpdateResource(ctx, res)
}
