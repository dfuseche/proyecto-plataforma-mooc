-- Tabla de Inscripciones de Estudiantes
CREATE TABLE IF NOT EXISTS enrollments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'withdrawn')),
    academic_status VARCHAR(32) NOT NULL DEFAULT 'enrolled' CHECK (academic_status IN ('enrolled', 'completed', 'approved')),
    progress_percentage NUMERIC(5,2) NOT NULL DEFAULT 0.00,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(student_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_enrollments_student ON enrollments(student_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_course ON enrollments(course_id);

-- Tabla de Quizzes
CREATE TABLE IF NOT EXISTS quizzes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    resource_id UUID NOT NULL REFERENCES course_resources(id) ON DELETE CASCADE,
    max_attempts INT NOT NULL DEFAULT 3,
    passing_score NUMERIC(5,2) NOT NULL DEFAULT 70.00,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_quizzes_resource ON quizzes(resource_id);

-- Tabla de Preguntas de Quizzes
CREATE TABLE IF NOT EXISTS quiz_questions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    quiz_id UUID NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    question_text TEXT NOT NULL,
    position INT NOT NULL,
    points NUMERIC(5,2) NOT NULL DEFAULT 1.00,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_questions_quiz ON quiz_questions(quiz_id);

-- Tabla de Opciones de Respuesta (is_correct NUNCA debe enviarse a respuestas HTTP de estudiantes)
CREATE TABLE IF NOT EXISTS quiz_options (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    question_id UUID NOT NULL REFERENCES quiz_questions(id) ON DELETE CASCADE,
    option_text TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT FALSE,
    feedback TEXT,
    position INT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_options_question ON quiz_options(question_id);

-- Tabla de Intentos de Quiz
CREATE TABLE IF NOT EXISTS quiz_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    quiz_id UUID NOT NULL REFERENCES quizzes(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'submitted', 'expired')),
    score NUMERIC(5,2),
    answers JSONB NOT NULL DEFAULT '{}'::jsonb,
    submitted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_attempts_student_quiz ON quiz_attempts(student_id, quiz_id);

-- Tabla de Progreso de Recursos Verificado en Servidor (Keyed por stable_id)
CREATE TABLE IF NOT EXISTS resource_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    resource_stable_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'opened' CHECK (status IN ('opened', 'completed')),
    dwell_time_seconds INT NOT NULL DEFAULT 0,
    last_position_seconds INT NOT NULL DEFAULT 0,
    last_heartbeat_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(student_id, course_id, resource_stable_id)
);

CREATE INDEX IF NOT EXISTS idx_progress_student_course ON resource_progress(student_id, course_id);
CREATE INDEX IF NOT EXISTS idx_progress_stable ON resource_progress(resource_stable_id);

-- Tabla de Insignias Verificables Emitidas (Idempotentes)
CREATE TABLE IF NOT EXISTS badges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    course_version_id UUID NOT NULL REFERENCES course_versions(id),
    verification_code UUID NOT NULL DEFAULT uuid_generate_v4(),
    image_url VARCHAR(1024) NOT NULL,
    verification_url VARCHAR(1024) NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    issued_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(student_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_badges_code ON badges(verification_code);
CREATE INDEX IF NOT EXISTS idx_badges_student ON badges(student_id);
