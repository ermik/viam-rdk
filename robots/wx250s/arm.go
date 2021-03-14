package wx250s

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"

	"go.viam.com/dynamixel/network"
	"go.viam.com/dynamixel/servo"
	"go.viam.com/dynamixel/servo/s_model"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/kinematics"
)

// SleepAngles are the angles we go to to prepare to turn off torque
var SleepAngles = map[string]float64{
	"Waist":       2048,
	"Shoulder":    840,
	"Elbow":       3090,
	"Forearm_rot": 2048,
	"Wrist":       2509,
	"Wrist_rot":   2048,
}

// OffAngles are the angles the arm falls into after torque is off
var OffAngles = map[string]float64{
	"Waist":       2048,
	"Shoulder":    795,
	"Elbow":       3091,
	"Forearm_rot": 2048,
	"Wrist":       2566,
	"Wrist_rot":   2048,
}

type Arm struct {
	Joints   map[string][]*servo.Servo
	moveLock *sync.Mutex
}

// servoPosToDegrees takes a 360 degree 0-4096 servo position, centered at 2048,
// and converts it to degrees, centered at 0
func servoPosToDegrees(pos float64) float64 {
	return ((pos - 2048) * 180) / 2048
}

// degreeToServoPos takes a 0-centered radian and converts to a 360 degree 0-4096 servo position, centered at 2048
func degreeToServoPos(pos float64) int {
	return int(2048 + (pos/180)*2048)
}

func NewArm(attributes api.AttributeMap, mutex *sync.Mutex) (api.Arm, error) {
	servos, err := findServos(attributes.GetString("usbPort"), attributes.GetString("baudRate"), attributes.GetString("armServoCount"))
	if err != nil {
		return nil, err
	}

	if mutex == nil {
		mutex = &sync.Mutex{}
	}

	newArm := &Arm{
		Joints: map[string][]*servo.Servo{
			"Waist":       {servos[0]},
			"Shoulder":    {servos[1], servos[2]},
			"Elbow":       {servos[3], servos[4]},
			"Forearm_rot": {servos[5]},
			"Wrist":       {servos[6]},
			"Wrist_rot":   {servos[7]},
		},
		moveLock: mutex,
	}

	return kinematics.NewArm(newArm, attributes.GetString("modelJSON"), 4)
}

func (a *Arm) CurrentPosition() (api.ArmPosition, error) {
	return api.ArmPosition{}, fmt.Errorf("wx250s dosn't support kinematics")
}

func (a *Arm) MoveToPosition(c api.ArmPosition) error {
	return fmt.Errorf("wx250s dosn't support kinematics")
}

// MoveToJointPositions takes a list of degrees and sets the corresponding joints to that position
func (a *Arm) MoveToJointPositions(jp api.JointPositions) error {
	if len(jp.Degrees) > len(a.JointOrder()) {
		return fmt.Errorf("passed in too many positions")
	}

	a.moveLock.Lock()

	// TODO(pl): make block configurable
	block := false
	for i, pos := range jp.Degrees {
		a.JointTo(a.JointOrder()[i], degreeToServoPos(pos), block)
	}

	a.moveLock.Unlock()
	return a.WaitForMovement()
}

// CurrentJointPositions returns an empty struct, because the wx250s should use joint angles from kinematics
func (a *Arm) CurrentJointPositions() (api.JointPositions, error) {
	return api.JointPositions{}, nil
}

func (a *Arm) JointMoveDelta(joint int, amount float64) error {
	return fmt.Errorf("not done yet")
}

// Close will get the arm ready to be turned off
func (a *Arm) Close() {
	// First, check if we are approximately in the sleep position
	// If so, we can just turn off torque
	// If not, let's move through the home position first
	angles, err := a.GetAllAngles()
	if err != nil {
		golog.Global.Errorf("failed to get angles: %s", err)
	}
	alreadyAtSleep := true
	for _, joint := range a.JointOrder() {
		if !within(angles[joint], SleepAngles[joint], 15) && !within(angles[joint], OffAngles[joint], 15) {
			alreadyAtSleep = false
		}
	}
	if !alreadyAtSleep {
		err = a.HomePosition()
		if err != nil {
			golog.Global.Errorf("Home position error: %s", err)
		}
	}
	err = a.SleepPosition()
	if err != nil {
		golog.Global.Errorf("Sleep pos error: %s", err)
	}
	err = a.TorqueOff()
	if err != nil {
		golog.Global.Errorf("Torque off error: %s", err)
	}
}

// GetAllAngles will return a map of the angles of each joint, denominated in servo position
func (a *Arm) GetAllAngles() (map[string]float64, error) {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	angles := make(map[string]float64)
	for jointName, servos := range a.Joints {
		angleSum := 0
		for _, s := range servos {
			pos, err := s.PresentPosition()
			if err != nil {
				return angles, err
			}
			angleSum += pos
		}
		angleMean := float64(angleSum / len(servos))
		angles[jointName] = angleMean
	}
	return angles, nil
}

func (a *Arm) JointOrder() []string {
	return []string{"Waist", "Shoulder", "Elbow", "Forearm_rot", "Wrist", "Wrist_rot"}
}

// Print positions of all servos
// TODO(pl): Print joint names, not just servo numbers
func (a *Arm) PrintPositions() error {
	posString := ""
	for i, s := range a.GetAllServos() {
		pos, err := s.PresentPosition()
		if err != nil {
			return err
		}
		posString = fmt.Sprintf("%s || %d : %d, %f degrees", posString, i, pos, servoPosToDegrees(float64(pos)))
	}
	return nil
}

// Return a slice containing all servos in the arm
func (a *Arm) GetAllServos() []*servo.Servo {
	var servos []*servo.Servo
	for _, joint := range a.JointOrder() {
		servos = append(servos, a.Joints[joint]...)
	}
	return servos
}

// Return a slice containing all servos in the named joint
func (a *Arm) GetServos(jointName string) []*servo.Servo {
	var servos []*servo.Servo
	servos = append(servos, a.Joints[jointName]...)
	return servos
}

// Set Acceleration for servos
func (a *Arm) SetAcceleration(accel int) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetProfileAcceleration(accel)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set Velocity for servos in travel time
// Recommended value 1000
func (a *Arm) SetVelocity(veloc int) error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetProfileVelocity(veloc)
		if err != nil {
			return err
		}
	}
	return nil
}

//Turn on torque for all servos
func (a *Arm) TorqueOn() error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetTorqueEnable(true)
		if err != nil {
			return err
		}
	}
	return nil
}

//Turn off torque for all servos
func (a *Arm) TorqueOff() error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	for _, s := range a.GetAllServos() {
		err := s.SetTorqueEnable(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set a joint to a position
func (a *Arm) JointTo(jointName string, pos int, block bool) {
	if pos > 4095 {
		pos = 4095
	} else if pos < 0 {
		pos = 0
	}

	err := servo.GoalAndTrack(pos, block, a.GetServos(jointName)...)
	if err != nil {
		golog.Global.Errorf("%s jointTo error: %s", jointName, err)
	}
}

//Go back to the sleep position, ready to turn off torque
func (a *Arm) SleepPosition() error {
	a.moveLock.Lock()
	sleepWait := false
	a.JointTo("Waist", 2048, sleepWait)
	a.JointTo("Shoulder", 840, sleepWait)
	a.JointTo("Wrist_rot", 2048, sleepWait)
	a.JointTo("Wrist", 2509, sleepWait)
	a.JointTo("Forearm_rot", 2048, sleepWait)
	a.JointTo("Elbow", 3090, sleepWait)
	a.moveLock.Unlock()
	return a.WaitForMovement()
}

func (a *Arm) GetMoveLock() *sync.Mutex {
	return a.moveLock
}

//Go to the home position
func (a *Arm) HomePosition() error {
	a.moveLock.Lock()

	wait := false
	for jointName := range a.Joints {
		a.JointTo(jointName, 2048, wait)
	}
	a.moveLock.Unlock()
	return a.WaitForMovement()
}

// WaitForMovement takes some servos, and will block until the servos are done moving
func (a *Arm) WaitForMovement() error {
	a.moveLock.Lock()
	defer a.moveLock.Unlock()
	allAtPos := false

	for !allAtPos {
		time.Sleep(200 * time.Millisecond)
		allAtPos = true
		for _, s := range a.GetAllServos() {
			isMoving, err := s.Moving()
			if err != nil {
				return err
			}
			// TODO(pl): Make this configurable
			if isMoving != 0 {
				allAtPos = false
			}
		}
	}
	return nil
}

func setServoDefaults(newServo *servo.Servo) error {
	// Set some nice-to-have settings
	//~ 	err := newServo.SetMovingThreshold(0)
	//~ 	if err != nil {
	//~ 		golog.Global.Fatalf("error SetMovingThreshold servo %d: %v\n", newServo.ID, err)
	//~ 	}
	err := newServo.SetPGain(2800)
	if err != nil {
		return fmt.Errorf("error SetPGain servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetIGain(50)
	if err != nil {
		return fmt.Errorf("error SetIGain servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetTorqueEnable(true)
	if err != nil {
		return fmt.Errorf("error SetTorqueEnable servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetProfileVelocity(50)
	if err != nil {
		return fmt.Errorf("error SetProfileVelocity servo %d: %v", newServo.ID, err)
	}
	err = newServo.SetProfileAcceleration(10)
	if err != nil {
		return fmt.Errorf("error SetProfileAcceleration servo %d: %v", newServo.ID, err)
	}
	return nil
}

// Find the specified number of Dynamixel servos on the specified USB port
// We're going to hardcode some USB parameters that we will literally never want to change
func findServos(usbPort, baudRateStr, armServoCountStr string) ([]*servo.Servo, error) {
	baudRate, err := strconv.Atoi(baudRateStr)
	if err != nil {
		return nil, fmt.Errorf("mangled baudrate: %v", err)
	}
	armServoCount, err := strconv.Atoi(armServoCountStr)
	if err != nil {
		return nil, fmt.Errorf("mangled servo count: %v", err)
	}

	options := serial.OpenOptions{
		PortName:              usbPort,
		BaudRate:              uint(baudRate),
		DataBits:              8,
		StopBits:              1,
		MinimumReadSize:       0,
		InterCharacterTimeout: 100,
	}

	serial, err := serial.Open(options)
	if err != nil {
		return nil, fmt.Errorf("error opening serial port: %v", err)
	}

	var servos []*servo.Servo

	network := network.New(serial)

	// By default, Dynamixel servos come 1-indexed out of the box because reasons
	for i := 1; i <= armServoCount; i++ {
		//Get model ID of each servo
		newServo, err := s_model.New(network, i)
		if err != nil {
			return nil, fmt.Errorf("error initializing servo %d: %v", i, err)
		}

		err = setServoDefaults(newServo)
		if err != nil {
			return nil, err
		}

		servos = append(servos, newServo)
	}

	return servos, nil
}

func within(a, b, c float64) bool {
	return math.Abs(a-b) <= c
}