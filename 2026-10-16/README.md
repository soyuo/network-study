# Go 동시성 프로그래밍

네트워크 및 시스템 소프트웨어 개발에 필요한 Go 동시성 프로그래밍(Concurrency)을 집중적으로 학습하고,
이를 활용한 Concurrent Task Processor 작은 프로젝트를 구현한다.

## 난이도

NORMAL

## 최대 점수

100점

## 요구사항
- 작업을 Channel로 전달한다
- Worker 수를 설정할 수 있다
- Worker가 작업을 병렬로 실행한다
- WaitGroup으로 작업 종료를 관리한다
- Context로 전체 처리를 취소할 수 있다
- 개별 작업에 Timeout을 적용한다
- 처리 결과와 오류를 수집한다
- 실행 중인 Worker의 종료를 안전하게 처리한다
- Race Detector 테스트를 통과한다

## 제출물

- 실행 가능한 Go 프로젝트 전체 소스
- 단위 테스트 및 Race Detector 실행 결과
- README.md (실행 방법, 설계, 테스트 방법)
- Worker 수에 따른 처리 시간 비교
- 동시성 문제를 어떻게 예방했는지 설명하는 설계 문서

## 선택 요구사항

- 처리 시간과 성공·실패 건수를 출력한다

## 사용 가능 언어

- Go

## 제약사항

- Worker Pool은 외부 라이브러리 없이 직접 구현
- context, sync, time, errors 등 표준 라이브러리 활용
- 모든 작업이 완료되거나 취소되면 Goroutine이 정상 종료되어야 함
- 이미 취소된 작업을 새로 시작하지 않도록 설계
- 결과 채널 종료 시점과 소유권을 명확하게 관리
- 작업 완료 순서는 제출 순서와 다를 수 있음

### 제출 결과

98/100점

### 1. Goroutine과 OS Thread의 차이

| 구분 | Goroutine | OS Thread |
| --- | --- | --- |
| 관리 주체 | Go Runtime | 운영체제 |
| 생성 비용 | 매우 작음 | 상대적으로 큼 |
| 메모리 | 작은 초기 스택, 필요 시 증가 | 더 큰 고정 스택 |
| 스케줄링 | Go Runtime이 관리 | OS가 관리 |
| 개수 | 수천~수백만 개 가능 | 상대적으로 제한적 |
| 실행 관계 | 여러 Goroutine이 적은 수의 OS Thread에서 실행 | CPU 코어에서 직접 실행 |

Go의 GOMAXPROCS 설정에 따라 여러 Goroutine이 여러 OS Thread에서 병렬 실행될 수 있습니다. 즉, Goroutine은 OS Thread 위에서 실행되는 가벼운 작업 단위입니다.

———

### 2. Channel과 Mutex는 언제 사용하는가?

#### Channel

Goroutine 사이에서 데이터를 전달하거나 작업을 분배할 때 사용합니다.

```go
jobs := make(chan Task)

go worker(jobs)
jobs <- task
```

적합한 상황:

- Worker Pool
- 생산자-소비자 구조
- Goroutine 간 작업 전달
- 처리 완료 신호 전달
- 파이프라인 구성

Channel은 “메모리를 공유하기보다 통신으로 공유한다”는 Go의 동시성 방식에 적합합니다.

#### Mutex

여러 Goroutine이 공유 데이터를 동시에 읽거나 수정할 때 사용합니다.

```go
mu.Lock()
count++
mu.Unlock()
```

적합한 상황:

- 공유 Map 보호
- 공유 카운터 보호
- 짧은 임계 영역 보호
- 데이터 구조에 대한 원자적 접근 보장

정리하면:

- 작업이나 데이터를 Goroutine 사이로 전달한다 → Channel
- 공유 변수를 직접 보호한다 → Mutex

둘 중 하나가 항상 더 좋은 것은 아니며, 상황에 따라 함께 사용할 수도 있습니다.

———

### 3. Context가 취소되었는데도 Goroutine이 계속 실행되는 이유

Context의 취소는 Goroutine을 강제로 종료하지 않습니다. Context는 단지 취소 신호를 전달할 뿐이고, Goroutine이 직접 그 신호를 확인해야 합니다.

예를 들어 다음 코드는 취소되어도 계속 실행될 수 있습니다.

```go
func worker(ctx context.Context) {
    for {
        doWork()
    }
}
```

ctx.Done()을 확인하지 않기 때문입니다.

올바른 방식은 다음과 같습니다.

```go
func worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            doWork()
        }
    }
}
```

또는 Blocking 작업을 수행할 때도 Context를 함께 확인해야 합니다.

```go
select {
case <-ctx.Done():
    return ctx.Err()
case result := <-resultCh:
    return result
}
```

계속 실행되는 주요 원인은 다음과 같습니다.

- ctx.Done()을 확인하지 않음
- 무한 루프에서 취소 상태를 검사하지 않음
- 채널 수신이나 Mutex 대기가 취소 가능하지 않음
- 호출한 하위 함수가 Context를 전달받지 않음
- 작업 함수가 Context를 무시함
- 외부 라이브러리나 시스템 호출이 취소를 지원하지 않음

———

### 4. go test -race 통과가 Deadlock이나 Goroutine 누수를 보장하는가?

보장하지 않습니다.

go test -race는 주로 여러 Goroutine이 같은 메모리에 동시에 접근할 때 발생하는 Data Race를 탐지합니다. 하지만 다음 문제까지 완전히 탐지하지는 못합니다.

- Deadlock
- Goroutine Leak
- 채널을 닫지 않아 영원히 대기하는 문제
- 종료 신호를 받지 못하는 Goroutine
- 특정 실행 순서에서만 발생하는 교착 상태
- 테스트가 끝난 뒤 남아 있는 Goroutine

예를 들어 다음 코드는 Data Race는 없지만 Deadlock이 발생합니다.

```go
func main() {
    ch := make(chan int)

    ch <- 1 // 수신자가 없으므로 영원히 대기
}
```

따라서 다음 검사를 함께 해야 합니다.

- go test -race ./...
- Timeout을 사용하는 테스트
- WaitGroup 종료 확인
- 결과 Channel 종료 확인
- 취소 후 Goroutine 수 비교
- runtime.NumGoroutine() 검사
- 필요하면 pprof로 Goroutine 상태 확인
