export const NUM_CAMERAS = 8
const FADE_DURATION_MS = 2000

export interface Snapshot {
	program: number
	preview: number
	fading: boolean
	tally: { [cam: number]: { program: boolean; preview: boolean } }
}

export class AtemState {
	private program = 1
	private preview = 2
	// Cam that's still tallied "program" alongside the new program cam, during a fade's 2s window.
	private extraProgramTally: number | null = null
	private fadeTimer: NodeJS.Timeout | null = null

	constructor(private readonly onChange: (snapshot: Snapshot) => void) {}

	getSnapshot(): Snapshot {
		const tally: Snapshot['tally'] = {}
		for (let cam = 1; cam <= NUM_CAMERAS; cam++) {
			tally[cam] = {
				program: cam === this.program || cam === this.extraProgramTally,
				preview: cam === this.preview,
			}
		}
		return { program: this.program, preview: this.preview, fading: this.extraProgramTally !== null, tally }
	}

	setLive(cam: number): void {
		this.clearFade()
		this.program = cam
		this.emit()
	}

	setPreview(cam: number): void {
		this.clearFade()
		this.preview = cam
		this.emit()
	}

	cut(): void {
		this.clearFade()
		;[this.program, this.preview] = [this.preview, this.program]
		this.emit()
	}

	fade(): void {
		this.clearFade()
		const outgoing = this.program
		;[this.program, this.preview] = [this.preview, this.program]
		this.extraProgramTally = outgoing
		this.emit()

		this.fadeTimer = setTimeout(() => {
			this.extraProgramTally = null
			this.fadeTimer = null
			this.emit()
		}, FADE_DURATION_MS)
	}

	private clearFade(): void {
		if (this.fadeTimer) {
			clearTimeout(this.fadeTimer)
			this.fadeTimer = null
		}
		this.extraProgramTally = null
	}

	private emit(): void {
		this.onChange(this.getSnapshot())
	}
}
